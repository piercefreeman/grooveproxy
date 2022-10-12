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

	mimic "github.com/refraction-networking/utls"

	"github.com/elazarl/goproxy"
)

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

func NewProxy() *goproxy.ProxyHttpServer {
	/*
	 * Create a new proxy with
	 * @pierce - primary modification for the mimic
	 */
	proxy := goproxy.NewProxyHttpServer()

	proxy.Tr.DialTLS = func(network, addr string) (net.Conn, error) {
		conn, err := net.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		config := &mimic.Config{ServerName: host}
		uconn := mimic.UClient(conn, config, mimic.HelloChrome_Auto)
		if err := uconn.Handshake(); err != nil {
			return nil, err
		}
		return uconn, nil
	}

	return proxy
}

func main() {
	var (
		verbose = flag.Bool("v", true, "should every proxy request be logged to stdout")
		port    = flag.Int("port", 8080, "proxy http listen address")
	)
	flag.Parse()

	// Set our own CA instead of the one that's default bundled with the proxy
	setCA("ca.crt", "ca.key")

	proxy := NewProxy()
	proxy.Verbose = *verbose

	// Our other implementations cache the certificates for some length of time, so we do the
	// same here for equality in benchmarking
	proxy.CertStore = NewOptimizedCertStore()

	if proxy.Verbose {
		log.Printf("Server starting up! - configured to listen on http interface %d - mimic", *port)
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
	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
		HandleConnect(goproxy.AlwaysMitm)
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
