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

	// Mapping of host->handler. Note that this is the actual "host" of the dial, might
	// be the end host itself or the proxy server.
	protocolMap  map[string]http.RoundTripper
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
		protocolMap:   make(map[string]http.RoundTripper),
		dialerSession: dialerSession,
	}
}

func getDialerAddress(url *url.URL) string {
	/*
	 * If a port has been provided explicitly, use this as part of the connection dialer
	 * Otherwise fallback to the default port for http/https
	 */
	host, port, err := net.SplitHostPort(url.Host)
	if err == nil {
		return net.JoinHostPort(host, port)
	}

	return net.JoinHostPort(url.Host, url.Scheme)
}

func urlToHost(url *url.URL) string {
	addr := getDialerAddress(url)

	var err error
	var host string
	if host, _, err = net.SplitHostPort(addr); err != nil {
		host = addr
	}

	return host
}

func wrapConnectionWithTLS(url *url.URL, rawConnection net.Conn) (*utls.UConn, error) {
	/*
	 * Wrap the connection with sensible TLS defaults
	 */

	// HelloChrome_Auto | HelloFirefox_Auto | HelloIOS_Auto
	host := urlToHost(url)
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

	var response *http.Response = nil
	responseValid := false

	for !responseValid {
		// Iterate the dialer until we hit on the correct value
		dialerDefinition := rt.dialerSession.NextDialer(dialerContext)
		if dialerDefinition == nil {
			return nil, errors.New("Exhausted dialers")
		}

		host := dialerDefinition.GetHost(req)

		rt.protocolLock.RLock()
		handler, ok := rt.protocolMap[host]
		rt.protocolLock.RUnlock()

		var err error

		if !ok {
			// We don't have a protocol for this host, so we need to figure it out
			// Note that there can be multiple `solveProtocol` inflight for the same URL
			// since we don't have a mutex around the host yet
			handler, err = rt.solveTransport(req, dialerDefinition)

			if err != nil {
				log.Printf("Unable to solve protocol for %s: %s", host, err)
				return nil, err
			}
			rt.protocolLock.Lock()
			rt.protocolMap[host] = handler
			rt.protocolLock.Unlock()
		}

		response, err = handler.RoundTrip(req)

		// This should be the return contents for the actual page
		// Allow 200 messages and 300s (redirects)
		// Anything in 400s or 500s is an error - note that we include 404 errors here as a resolution error
		if err == nil && response.StatusCode >= 200 && response.StatusCode < 400 {
			responseValid = true
		} else {
			log.Printf("Invalid response for %s", host)
		}
	}

	return response, nil
}

func (rt *CustomRoundTripper) solveTransport(
	request *http.Request,
	dialerDefinition *DialerDefinition,
) (http.RoundTripper, error) {
	// Assume that the protocol will be constant for this given host, which is a reasonable
	// assumption given the lifespan of the proxy
	// TODO: Might consider invalidating the transport if we see errors with the connection
	protocol, _, err := rt.solveProtocol(request, dialerDefinition)
	log.Printf("Using protocol %d for %s", protocol, request.URL.Host)

	if err != nil {
		return nil, err
	}

	mainDialer := func(network, addr string) (net.Conn, error) {
		// Create a new connection with the protocol we know
		connection, err := dialerDefinition.Dial("tcp", getDialerAddress(request.URL))
		if err != nil {
			log.Printf("Unable to create connection for %s: %s", request.URL.Host, err)
			return nil, err
		}

		// If we have a TLS connection, we need to perform the handshake and wrap the connection
		if protocol == ProtocolHTTP1TLS || protocol == ProtocolHTTP2TLS {
			connection, err = wrapConnectionWithTLS(request.URL, connection)
			if err != nil {
				log.Printf("Unable to wrap connection for %s: %s", request.URL.Host, err)
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

func (rt *CustomRoundTripper) solveProtocol(request *http.Request, dialerDefinition *DialerDefinition) (int, net.Conn, error) {
	/*
	 * Solve the protocol for a given host
	 * We also pass along the connection for the solved stream in case clients want to immediately
	 * start using an open connection
	 */
	// Create a new connection
	rawConnection, error := dialerDefinition.Dial("tcp", getDialerAddress(request.URL))

	if error != nil {
		return -1, nil, error
	}

	// If the request is "http" assume we're using HTTP/1.1 since HTTP/2 is only supported over TLS
	if strings.ToLower(request.URL.Scheme) == "http" {
		log.Printf("Using HTTP/1.1 for %s", request.URL.Host)
		return ProtocolHTTP1, rawConnection, nil
	}

	// Attempt to perform the TLS connection
	connection, err := wrapConnectionWithTLS(request.URL, rawConnection)
	if err != nil {
		return -1, nil, err
	}

	// Check if we have a successful TLS connection
	if connection.ConnectionState().HandshakeComplete {
		// Check if we have a HTTP2 connection
		if connection.ConnectionState().NegotiatedProtocol == http2.NextProtoTLS {
			log.Printf("Using HTTP/2TLS for %s", request.URL.Host)
			return ProtocolHTTP2TLS, connection, nil
		} else {
			log.Printf("Using HTTP/1TLS for %s", request.URL.Host)
			return ProtocolHTTP1TLS, connection, nil
		}
	}

	return -1, nil, errors.New("Unable to solve protocol")
}
