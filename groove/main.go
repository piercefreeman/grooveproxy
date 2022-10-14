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

	"github.com/gin-gonic/gin"
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

	r := gin.Default()
	r.GET("/api/tape/record", func(c *gin.Context) {
		// Start to record the requests
		recorder.Print()

		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	var (
		verbose = flag.Bool("v", true, "should every proxy request be logged to stdout")
		port    = flag.Int("port", 6010, "proxy http listen address")
	)
	flag.Parse()

	log.Printf("Verbose: %v", *verbose)

	// Set our own CA instead of the one that's default bundled with the proxy
	setCA("ssl/ca.crt", "ssl/ca.key")

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

	/*proxy.OnRequest().DoFunc(
		// Request logger
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			// TODO: Bail if `RecorderModeOff`

			body, err := io.ReadAll(r.Body)

			if err != nil {
				log.Println("Unable to read body stream")
				return r, nil
			}
			r.Body = ioutil.NopCloser(bytes.NewReader(body))

			log.Println("Add record log.")
			recorder.requests = append(
				recorder.requests,
				&RecordedRecord{
					request: ArchivedRequest{
						url:     r.Host,
						method:  r.Method,
						headers: r.Header,
						body:    body,
						order:   0,
					},
				},
			)

			return r, nil
		},
	)*/

	proxy.OnResponse().DoFunc(
		func(response *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			log.Println("handle response...")
			requestHistory, responseHistory := getRedirectHistory(response)

			// When replaying this we want to replay it in order to capture all the
			// event history and test redirect handlers
			for i := 0; i < len(requestHistory); i++ {
				request := requestHistory[i]
				response := responseHistory[i]

				log.Println("Add record log.")
				recorder.LogPair(request, response)
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
		r.Run(":5010")
	}()

	go func() {
		log.Fatalln(http.ListenAndServe(":"+strconv.Itoa(*port), proxy))
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)

	<-sigc

	log.Println("goproxy: shutting down")
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
