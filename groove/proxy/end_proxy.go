package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"

	goproxy "github.com/piercefreeman/goproxy"
)

type EndProxy struct {
	proxy *goproxy.ProxyHttpServer

	// Sets a dial function for use when the proxy is set
	// When ProxyDial is non-nil, the built-in dialer will attempt to pass the dial through
	// the proxy. Otherwise it will use the normal dialer.
	ProxyDial func(network, addr string) (net.Conn, error)

	// Sets an optional request function callback when the proxy is set
	RequestFunction func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response)
}

const ProxyAuthHeader = "Proxy-Authorization"

func SetBasicAuth(username, password string, req *http.Request) {
	req.Header.Set(ProxyAuthHeader, fmt.Sprintf("Basic %s", basicAuth(username, password)))
}

func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

func newEndProxy(proxy *goproxy.ProxyHttpServer) *EndProxy {
	return &EndProxy{
		proxy: proxy,
	}
}

func (endproxy *EndProxy) disableProxy() {
	endproxy.proxy.Tr.Proxy = nil
	endproxy.ProxyDial = nil
	endproxy.RequestFunction = nil
}

func (endproxy *EndProxy) updateProxy(proxyUrl string, username string, password string) {
	/*
	 * To create a end proxy connection without username and password, pass a blank
	 * string for these values.
	 */
	endproxy.proxy.Tr.Proxy = func(req *http.Request) (*url.URL, error) {
		return url.Parse(proxyUrl)
	}

	if len(username) > 0 && len(password) > 0 {
		log.Println("Continuing with authenticated end proxy...")

		connectReqHandler := func(req *http.Request) {
			log.Printf("Will set basic auth %s %s\n", username, password)
			SetBasicAuth(username, password, req)
		}
		endproxy.ProxyDial = endproxy.proxy.NewConnectDialToProxyWithHandler(proxyUrl, connectReqHandler)

		endproxy.RequestFunction = func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			log.Printf("Will set basic auth request %s %s\n", username, password)
			SetBasicAuth(username, password, req)
			return req, nil
		}

	} else {
		log.Println("Continuing with unauthenticated end proxy...")
		endproxy.ProxyDial = endproxy.proxy.NewConnectDialToProxy(proxyUrl)
		endproxy.RequestFunction = nil
	}
}

func (endproxy *EndProxy) setupMiddleware(roundTripper *roundTripper) {
	endproxy.proxy.OnRequest().Do(goproxy.FuncReqHandler(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if endproxy.RequestFunction != nil {
			return endproxy.RequestFunction(req, ctx)
		}
		return req, nil
	}))

	endproxy.proxy.ConnectDial = endproxy.dynamicEndDialer
	roundTripper.Dialer = endproxy.dynamicEndDialer
}

func (endproxy *EndProxy) dynamicEndDialer(network, addr string) (net.Conn, error) {
	// If proxy is set, route to proxy
	// otherwise use typical network dialer
	if endproxy.ProxyDial != nil {
		return endproxy.ProxyDial(network, addr)
	} else {
		return net.Dial(network, addr)
	}
}
