// https://github.com/elazarl/goproxy/blob/master/examples/goproxy-transparent/transparent.go
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"

	"github.com/piercefreeman/goproxy"
)

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

func getRedirectHistory(response *http.Response) ([]*http.Request, []*http.Response) {
	// The eventually resolved response payload carries alongside all of the request
	// history - this function reassembles it
	requestHistory := make([]*http.Request, 0)
	responseHistory := make([]*http.Response, 0)

	for response != nil {
		request := response.Request
		requestHistory = append(requestHistory, request)
		responseHistory = append(responseHistory, response)
		response = request.Response
	}

	// The response order is actually reversed from what we expect
	// The last request that eventually made the response comes first in the slice
	reverseSlice(requestHistory)
	reverseSlice(responseHistory)

	return requestHistory, responseHistory
}

func main() {
	recorder := NewRecorder()

	var (
		verbose     = flag.Bool("v", true, "should every proxy request be logged to stdout")
		port        = flag.Int("port", 6010, "proxy http listen address")
		controlPort = flag.Int("control-port", 5010, "control API listen address")
	)
	flag.Parse()

	log.Printf("Verbose: %v", *verbose)

	// Set our own CA instead of the one that's default bundled with the proxy
	setCA("ssl/ca.crt", "ssl/ca.key")

	controller := createController(recorder)

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = *verbose

	// Our other implementations cache the certificates for some length of time, so we do the
	// same here for equality in benchmarking
	proxy.CertStore = NewOptimizedCertStore()

	// Fingerprint mimic logic
	proxy.RoundTripper = newRoundTripper()

	if proxy.Verbose {
		log.Printf("Server starting up! - configured to listen on http interface %d", *port)
	}

	proxy.NonproxyHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host == "" {
			fmt.Fprintln(w, "Cannot handle requests without Host header, e.g., HTTP 1.0")
			return
		}
		req.URL.Scheme = "http"
		req.URL.Host = req.Host
		proxy.ServeHTTP(w, req)
	})

	proxy.OnRequest().DoFunc(
		/*
		 * Recorder
		 */
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			// Only handle responses during write mode
			if recorder.mode != RecorderModeRead {
				return r, nil
			}

			recordResult := recorder.FindMatchingResponse(r)

			if recordResult != nil {
				log.Printf("Record found: %s\n", r.URL.String())
				return r, recordResult
			} else {
				log.Printf("No matching record found: %s\n", r.URL.String())
				// Implementation specific - for now fail result if we can't find a
				// playback entry in the tape
				return r, goproxy.NewResponse(
					r,
					goproxy.ContentTypeText,
					http.StatusInternalServerError,
					"Proxy blocked request",
				)
			}

			// Passthrough
			//return r, nil
		})

	proxy.OnResponse().DoFunc(
		/*
		 * Recorder
		 */
		func(response *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			// Only handle responses during write mode
			if recorder.mode != RecorderModeWrite {
				return response
			}

			log.Println("Record request/response...")
			requestHistory, responseHistory := getRedirectHistory(response)

			// When replaying this we want to replay it in order to capture all the
			// event history and test redirect handlers
			for i := 0; i < len(requestHistory); i++ {
				request := requestHistory[i]
				response := responseHistory[i]

				recorder.LogPair(request, response)
				log.Println("Added record log.")
			}

			return response
		},
	)

	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
		HandleConnect(goproxy.AlwaysMitm)

	// https://github.com/elazarl/goproxy/blob/master/examples/goproxy-eavesdropper/main.go
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*:80$"))).
		HijackConnect(func(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
			defer func() {
				if e := recover(); e != nil {
					ctx.Logf("error connecting to remote: %v", e)
					client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
				}
				client.Close()
			}()
			clientBuf := bufio.NewReadWriter(bufio.NewReader(client), bufio.NewWriter(client))

			remote, err := connectDial(req.Context(), proxy, "tcp", req.URL.Host)
			orPanic(err)
			remoteBuf := bufio.NewReadWriter(bufio.NewReader(remote), bufio.NewWriter(remote))
			for {
				req, err := http.ReadRequest(clientBuf.Reader)
				orPanic(err)
				orPanic(req.Write(remoteBuf))
				orPanic(remoteBuf.Flush())
				resp, err := http.ReadResponse(remoteBuf.Reader, req)
				orPanic(err)
				orPanic(resp.Write(clientBuf.Writer))
				orPanic(clientBuf.Flush())
			}
		})

	go func() {
		controller.Run(":" + strconv.Itoa(*controlPort))
	}()

	go func() {
		log.Fatalln(http.ListenAndServe(":"+strconv.Itoa(*port), proxy))
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)

	<-sigc

	log.Println("groove: shutting down")
	os.Exit(0)
}

// copied/converted from https.go
func dial(ctx context.Context, proxy *goproxy.ProxyHttpServer, network, addr string) (c net.Conn, err error) {
	if proxy.Tr.DialContext != nil {
		return proxy.Tr.DialContext(ctx, network, addr)
	}
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

// copied/converted from https.go
func connectDial(ctx context.Context, proxy *goproxy.ProxyHttpServer, network, addr string) (c net.Conn, err error) {
	if proxy.ConnectDial == nil {
		return dial(ctx, proxy, network, addr)
	}
	return proxy.ConnectDial(network, addr)
}
