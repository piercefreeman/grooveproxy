// https://github.com/elazarl/goproxy/blob/master/examples/goproxy-transparent/transparent.go
package main

import (
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

func main() {
	if len(os.Args) > 1 {
		command := os.Args[1]
		if command == "install-ca" {
			installCA()
			return
		} else {
			// Assume other requests will be handled by the regular proxy - passthrough
		}
	}

	var (
		verbose     = flag.Bool("v", true, "should every proxy request be logged to stdout")
		port        = flag.Int("port", 6010, "proxy http listen address")
		controlPort = flag.Int("control-port", 6011, "control API listen address")

		// Location to CA
		caCertificate = flag.String("ca-certificate", "", "Path to CA Certificate")
		caKey         = flag.String("ca-key", "", "Path to CA Key")

		// Cache size (in memory)
		cacheMemorySize = flag.Int("cache-memory-mb", 25, "cache memory size")

		// Require authentication to access this proxy
		//authUsername = flag.String("auth-username", "", "Require authentication to the current server")
		//authPassword = flag.String("auth-password", "", "Require authentication to the current server")
	)
	flag.Parse()

	log.Printf("Verbose: %v", *verbose)

	if len(*caCertificate) == 0 || len(*caKey) == 0 {
		log.Println("Falling back to default CA certificate")
		_, localCAPath, localCAKey := getLocalCAPaths()
		if err := setCA(localCAPath, localCAKey); err != nil {
			log.Fatal(fmt.Errorf("Error setting CA: %w", err))
		}
	} else {
		// Set our own CA instead of the one that's default bundled with the proxy
		if err := setCA(*caCertificate, *caKey); err != nil {
			log.Fatal(fmt.Errorf("Error setting CA: %w", err))
		}
	}

	recorder := NewRecorder()
	cache := NewCache(uint64(*cacheMemorySize))

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = *verbose

	// If specified, protect the proxy with an auth login
	// @pierce - Currently failing on MITM because of repeat CONNECTs, some without auth
	/*if len(*authUsername) > 0 && len(*authPassword) > 0 {
		log.Println("Protect proxy with username and password")
		auth.ProxyBasicMitm(proxy, "my_realm", func(user, pwd string) bool {
			return user == *authUsername && pwd == *authPassword
		})
	} else {
		log.Println("Creating unauthenticated proxy")
	}*/

	// Our other implementations cache the certificates for some length of time, so we do the
	// same here for equality in benchmarking
	proxy.CertStore = NewOptimizedCertStore()

	dialerSession := NewDialerSession()

	// Default the session to a full passthrough from local -> Internet
	// This will get overridden by clients when they provide values
	dialerSession.DialerDefinitions = append(
		dialerSession.DialerDefinitions,
		NewDialerDefinition(0, nil, nil),
	)

	roundTripper := NewCustomRoundTripper(dialerSession)

	// Static function to run a new dial without the context of a particular request
	// In theory we could just make this static at launch time but we keep it dynamic
	// in case the `next` logic changes inflight (ie. clients add more proxies with
	// different priorities, etc) - there are some slight performance impacts to this
	// approach but it's likely negligible given the overall network latencies
	proxy.ConnectDial = func(network, addr string) (net.Conn, error) {
		context := dialerSession.NewDialerContext(nil)
		dialDefinition := dialerSession.NextDialer(context)
		return dialDefinition.Dial(network, addr)
	}

	controller := createController(recorder, cache, dialerSession)

	// Cast the custom roundtripper implementation to a standard http.RoundTripper
	proxy.RoundTripper = http.RoundTripper(roundTripper)

	if proxy.Verbose {
		log.Printf("Server starting up! - configured to listen on http interface %d", *port)
	}

	setupHeadersMiddleware(proxy)
	setupRecorderMiddleware(proxy, recorder)
	setupCacheMiddleware(proxy, cache, recorder)

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

	go func() {
		controller.Run(":" + strconv.Itoa(*controlPort))
	}()

	go func() {
		// Host on TLS so clients can use http/2 multiplexing - required for the requests
		// that block the system lock
		log.Fatalln(http.ListenAndServe(":"+strconv.Itoa(*port), proxy))
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)

	<-sigc

	log.Println("groove: shutting down")
	os.Exit(0)
}
