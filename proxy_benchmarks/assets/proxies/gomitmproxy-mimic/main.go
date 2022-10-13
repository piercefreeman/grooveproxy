// https://github.com/AdguardTeam/gomitmproxy/blob/master/examples/mitm/main.go
package main

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/piercefreeman/gomitmproxy"

	"github.com/piercefreeman/gomitmproxy/mitm"

	"crypto/tls"

	"github.com/AdguardTeam/golibs/log"
	"github.com/piercefreeman/gomitmproxy/proxyutil"
)

func main() {
	log.SetLevel(log.DEBUG)

	port := flag.Int("port", 8080, "bind port")
	flag.Parse()

	go func() {
		// TODO: Random port?
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// READ CERT AND KEY
	tlsCert, err := tls.LoadX509KeyPair("ca.crt", "ca.key")
	if err != nil {
		log.Fatal(err)
	}
	privateKey := tlsCert.PrivateKey.(*rsa.PrivateKey)

	x509c, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		log.Fatal(err)
	}

	mitmConfig, err := mitm.NewConfig(x509c, privateKey, &CustomCertsStorage{
		certsCache: map[string]*tls.Certificate{}},
	)
	if err != nil {
		log.Fatal(err)
	}

	mitmConfig.SetValidity(time.Hour * 24 * 7) // generate certs valid for 7 days
	mitmConfig.SetOrganization("gomitmproxy")  // cert organization

	// PREPARE PROXY
	addr := &net.TCPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: *port,
	}

	proxy := gomitmproxy.NewProxy(gomitmproxy.Config{
		ListenAddr: addr,

		APIHost: "gomitmproxy",

		MITMConfig: mitmConfig,
		// MITMExceptions: []string{"example.com"},

		OnRequest:  onRequest,
		OnResponse: onResponse,
		OnConnect:  onConnect,
	})

	// Handle our http/1 - http/2 and mimic logic
	proxy.Transport = newRoundTripper()

	err = proxy.Start()
	if err != nil {
		log.Fatal(err)
	}

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalChannel

	fmt.Println("Shutting down proxy...")

	// CLOSE THE PROXY
	proxy.Close()
}

func onRequest(session *gomitmproxy.Session) (*http.Request, *http.Response) {
	req := session.Request()

	log.Printf("onRequest: %s %s", req.Method, req.URL.String())

	if req.URL.Host == "example.net" {
		body := strings.NewReader("<html><body><h1>Replaced response</h1></body></html>")
		res := proxyutil.NewResponse(http.StatusOK, body, req)
		res.Header.Set("Content-Type", "text/html")
		session.SetProp("blocked", true)
		return nil, res
	}

	if req.URL.Host == "testgomitmproxy" {
		body := strings.NewReader("<html><body><h1>Served by gomitmproxy</h1></body></html>")
		res := proxyutil.NewResponse(http.StatusOK, body, req)
		res.Header.Set("Content-Type", "text/html")
		return nil, res
	}

	return nil, nil
}

func onResponse(session *gomitmproxy.Session) *http.Response {
	log.Printf("onResponse: %s", session.Request().URL.String())

	if _, ok := session.GetProp("blocked"); ok {
		log.Printf("onResponse: was blocked")
		return nil
	}

	res := session.Response()
	req := session.Request()

	if strings.Index(res.Header.Get("Content-Type"), "text/html") != 0 {
		// Do nothing with non-HTML responses
		return nil
	}

	b, err := proxyutil.ReadDecompressedBody(res)
	// Close the original body
	_ = res.Body.Close()
	if err != nil {
		return proxyutil.NewErrorResponse(req, err)
	}

	// Use latin1 before modifying the body
	// Using this 1-byte encoding will let us preserve all original characters
	// regardless of what exactly is the encoding
	body, err := proxyutil.DecodeLatin1(bytes.NewReader(b))
	if err != nil {
		return proxyutil.NewErrorResponse(session.Request(), err)
	}

	// Modifying the original body
	modifiedBody, err := proxyutil.EncodeLatin1(body + "<!-- EDITED -->")
	if err != nil {
		return proxyutil.NewErrorResponse(session.Request(), err)
	}

	res.Body = ioutil.NopCloser(bytes.NewReader(modifiedBody))
	res.Header.Del("Content-Encoding")
	res.ContentLength = int64(len(modifiedBody))
	return res
}

func onConnect(session *gomitmproxy.Session, proto string, addr string) net.Conn {
	host, _, err := net.SplitHostPort(addr)

	if err == nil && host == "testgomitmproxy" {
		// Don't let it connecting there -- we'll serve it by ourselves
		return &proxyutil.NoopConn{}
	}

	return nil
}

// CustomCertsStorage - an example of a custom cert storage
type CustomCertsStorage struct {
	certsCache map[string]*tls.Certificate // cache with the generated certificates
}

// Get gets the certificate from the storage
func (c *CustomCertsStorage) Get(key string) (*tls.Certificate, bool) {
	v, ok := c.certsCache[key]
	return v, ok
}

// Set saves the certificate to the storage
func (c *CustomCertsStorage) Set(key string, cert *tls.Certificate) {
	c.certsCache[key] = cert
}
