package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func main() {
	var (
		port    = flag.Int("port", 3010, "http port to listen on")
		tlsPort = flag.Int("tls-port", 3011, "tls port to listen on")
	)
	flag.Parse()

	http.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New()

		time.Sleep(1 * time.Second)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Request handled.\nValue:" + id.String()))
	})

	fmt.Printf("Will launch speed test server on port %d and tls on %d\n", *port, *tlsPort)

	go func() {
		err := http.ListenAndServe(":"+strconv.Itoa(*port), nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		// generate a `Certificate` struct
		cert, _ := tls.LoadX509KeyPair("ssl/cert.crt", "ssl/cert.key")

		// create a custom server with `TLSConfig`
		s := &http.Server{
			Addr:    ":" + strconv.Itoa(*tlsPort),
			Handler: nil, // use `http.DefaultServeMux`
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		}

		//err := http.ListenAndServeTLS(":"+strconv.Itoa(tlsPort), "cert.crt", "cert.key", nil)
		err := s.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatal(err)
		}
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)

	<-sigc

	log.Println("speed test: shutting down")
	os.Exit(0)
}
