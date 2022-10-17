package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/url"

	goproxy "github.com/piercefreeman/goproxy"
)

const ProxyAuthHeader = "Proxy-Authorization"

func SetBasicAuth(username, password string, req *http.Request) {
	req.Header.Set(ProxyAuthHeader, fmt.Sprintf("Basic %s", basicAuth(username, password)))
}

func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

func setupEndProxyMiddleware(
	proxy *goproxy.ProxyHttpServer,
	roundTripper *roundTripper,
	proxyUrl string,
	username string,
	password string,
) {
	/*
	 * To create a end proxy connection without username and password, pass a blank
	 * string for these values.
	 */
	proxy.Tr.Proxy = func(req *http.Request) (*url.URL, error) {
		return url.Parse(proxyUrl)
	}
	if len(username) > 0 && len(password) > 0 {
		log.Println("Continuing with authenticated end proxy...")

		connectReqHandler := func(req *http.Request) {
			log.Printf("Will set basic auth %s %s\n", username, password)
			SetBasicAuth(username, password, req)
		}
		proxy.ConnectDial = proxy.NewConnectDialToProxyWithHandler(proxyUrl, connectReqHandler)
		roundTripper.Dialer = proxy.NewConnectDialToProxyWithHandler(proxyUrl, connectReqHandler)
		proxy.OnRequest().Do(goproxy.FuncReqHandler(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			log.Printf("Will set basic auth request %s %s\n", username, password)
			SetBasicAuth(username, password, req)
			return req, nil
		}))
	} else {
		log.Println("Continuing with unauthenticated end proxy...")
		proxy.ConnectDial = proxy.NewConnectDialToProxy(proxyUrl)
		roundTripper.Dialer = proxy.NewConnectDialToProxy(proxyUrl)
	}
}
