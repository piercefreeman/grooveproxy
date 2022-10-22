package main

import (
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

type CustomRoundTripper struct {
	/*
	 * Generator session for global round-trips
	 * - Support utls handshakes with remote server
	 */

	// Mapping of dialer definition identifier->handler. Note that this is the actual "host" of the dial, might
	// be the end host itself or the proxy server.
	handlerMap  map[string]map[int]http.RoundTripper
	handlerLock sync.RWMutex

	// Host protocols are probably the same across definitions, but this might not be true
	// depending on the MITM properties of the middle proxy. For now we assume that it is in order
	// to avoid having to make a separate protocol lookup for each dial definition
	// TODO: Add fallback handling if we fail a lookup by leveraging an outdated host->protocol mapping
	// [definition][host] -> protocol
	protocolMap  map[string]int
	protocolLock sync.RWMutex

	// Dialer session
	dialerSession *DialerSession
}

const (
	ProtocolHTTP1    = iota
	ProtocolHTTP1TLS = iota
	ProtocolHTTP2TLS = iota
)

func NewCustomRoundTripper(dialerSession *DialerSession) *CustomRoundTripper {
	return &CustomRoundTripper{
		handlerMap:    make(map[string]map[int]http.RoundTripper),
		protocolMap:   make(map[string]int),
		dialerSession: dialerSession,
	}
}

func getDialerAddress(url *url.URL) string {
	/*
	 * If a port has been provided explicitly, use this as part of the connection dialer
	 * Otherwise fallback to the default port for http/https
	 * returns host:port (no scheme prefix)
	 */
	host, port, err := net.SplitHostPort(url.Host)
	if err == nil {
		return net.JoinHostPort(host, port)
	}

	return net.JoinHostPort(url.Host, url.Scheme)
}

func addressToHost(addr string) string {
	var err error
	var host string
	if host, _, err = net.SplitHostPort(addr); err != nil {
		host = addr
	}

	return host
}

func wrapConnectionWithTLS(host string, rawConnection net.Conn) (*utls.UConn, error) {
	/*
	 * Wrap the connection with sensible TLS defaults
	 */

	// HelloChrome_Auto | HelloFirefox_Auto | HelloIOS_Auto
	log.Printf("Performing TLS handshake with server name %s", host)
	connection := utls.UClient(rawConnection, &utls.Config{ServerName: host}, utls.HelloChrome_Auto)

	if err := connection.Handshake(); err != nil {
		log.Println("Handshake failed")
		connection.Close()
		return nil, err
	}

	return connection, nil
}

func (rt *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	/*
	 * Implement our custom roundtrip logic
	 * This is the only function that's actually required by the http.RoundTripper interface
	 */
	// New request, fresh context to track requests
	dialerContext := rt.dialerSession.NewDialerContext(req)

	// Remove additional headers that `removeProxyHeaders` doesn't cover
	req.Header.Del(ProxyResourceType)

	var response *http.Response = nil
	responseValid := false
	log.Printf("Requesting %s", req.URL.String())

	for !responseValid {
		// Iterate the dialer until we hit on the correct value
		dialerDefinition := rt.dialerSession.NextDialer(dialerContext)
		if dialerDefinition == nil {
			return nil, errors.New("Exhausted dialers")
		}

		protocol, err := rt.solveProtocol(req, dialerDefinition)
		if err != nil {
			log.Printf("Failed to solve protocol for %s: %s", req.URL.Host, err)
			continue
		}
		handler, err := rt.solveTransport(protocol, dialerDefinition)
		if err != nil {
			log.Printf("Failed to solve transport for %s: %s", dialerDefinition.identifier, err)
			continue
		}

		response, err = handler.RoundTrip(req)

		// This should be the return contents for the actual page
		// Allow 200 messages and 300s (redirects)
		// Anything in 400s or 500s is an error - note that we include 404 errors here as a resolution error
		if err == nil && response.StatusCode >= 200 && response.StatusCode < 400 {
			responseValid = true
		} else {
			log.Printf("Invalid response for %s", req.URL.String())
		}
	}

	return response, nil
}

func (rt *CustomRoundTripper) solveTransport(
	protocol int,
	dialerDefinition *DialerDefinition,
) (http.RoundTripper, error) {
	rt.handlerLock.RLock()
	handler, ok := rt.handlerMap[dialerDefinition.identifier][protocol]
	rt.handlerLock.RUnlock()

	if ok {
		log.Println("Cache hit: transport")
		return handler, nil
	}

	// We don't have a handler for this protocol, so we need to figure it out
	var err error
	handler, err = rt.solveTransportNew(protocol, dialerDefinition)

	if err != nil {
		log.Printf("Unable to solve transport for %s: %s", dialerDefinition.identifier, err)
		return nil, err
	}

	rt.handlerLock.Lock()
	if _, ok := rt.handlerMap[dialerDefinition.identifier]; !ok {
		rt.handlerMap[dialerDefinition.identifier] = make(map[int]http.RoundTripper)
	}
	rt.handlerMap[dialerDefinition.identifier][protocol] = handler
	rt.handlerLock.Unlock()

	return handler, nil
}

func (rt *CustomRoundTripper) solveProtocol(request *http.Request, dialerDefinition *DialerDefinition) (int, error) {
	host := getDialerAddress(request.URL)

	rt.protocolLock.RLock()
	protocol, ok := rt.protocolMap[host]
	rt.protocolLock.RUnlock()

	if ok {
		log.Println("Cache hit: protocol")
		return protocol, nil
	}

	// We don't have a protocol for this host, so we need to figure it out
	var err error
	protocol, err = rt.solveProtocolNew(request, dialerDefinition)

	if err != nil {
		return -1, err
	}

	rt.protocolLock.Lock()
	rt.protocolMap[host] = protocol
	rt.protocolLock.Unlock()

	return protocol, nil
}

func (rt *CustomRoundTripper) solveTransportNew(
	protocol int,
	dialerDefinition *DialerDefinition,
) (http.RoundTripper, error) {
	mainDialer := func(network, addr string) (net.Conn, error) {
		// Create a new connection with the protocol we know
		connection, err := dialerDefinition.Dial(network, addr)
		if err != nil {
			log.Printf("Unable to create connection for %s: %s", addr, err)
			return nil, err
		}

		// If we have a TLS connection, we need to perform the handshake and wrap the connection
		if protocol == ProtocolHTTP1TLS || protocol == ProtocolHTTP2TLS {
			connection, err = wrapConnectionWithTLS(addressToHost(addr), connection)
			if err != nil {
				log.Printf("Unable to wrap connection for %s: %s", addr, err)
				return nil, err
			}
		}

		return connection, nil
	}

	mainDialerHTTP2 := func(network, addr string, cfg *tls.Config) (net.Conn, error) {
		return mainDialer(network, addr)
	}

	var transport http.RoundTripper

	if protocol == ProtocolHTTP1 {
		transport = &http.Transport{Dial: mainDialer}
	} else if protocol == ProtocolHTTP1TLS {
		transport = &http.Transport{DialTLS: mainDialer}
	} else if protocol == ProtocolHTTP2TLS {
		transport = &http2.Transport{DialTLS: mainDialerHTTP2}
	}

	return transport, nil
}

func (rt *CustomRoundTripper) solveProtocolNew(request *http.Request, dialerDefinition *DialerDefinition) (int, error) {
	/*
	 * Solve the protocol for a given host
	 * We also pass along the connection for the solved stream in case clients want to immediately
	 * start using an open connection
	 */
	// Create a new connection
	rawConnection, err := dialerDefinition.Dial("tcp", getDialerAddress(request.URL))

	if err != nil {
		return -1, err
	}

	// If the request is "http" assume we're using HTTP/1.1 since HTTP/2 is only supported over TLS
	if strings.ToLower(request.URL.Scheme) == "http" {
		log.Printf("Using HTTP/1.1 for %s", request.URL.Host)
		return ProtocolHTTP1, nil
	}

	// Attempt to perform the TLS connection
	connection, err := wrapConnectionWithTLS(addressToHost(getDialerAddress(request.URL)), rawConnection)
	if err != nil {
		return -1, err
	}

	// Check if we have a successful TLS connection
	if connection.ConnectionState().HandshakeComplete {
		// Check if we have a HTTP2 connection
		if connection.ConnectionState().NegotiatedProtocol == http2.NextProtoTLS {
			log.Printf("Using HTTP/2TLS for %s", request.URL.Host)
			return ProtocolHTTP2TLS, nil
		} else {
			log.Printf("Using HTTP/1TLS for %s", request.URL.Host)
			return ProtocolHTTP1TLS, nil
		}
	}

	return -1, errors.New("Unable to solve protocol")
}
