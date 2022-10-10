package main

import (
	"crypto/tls"
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
	port := 3010
	tlsPort := 3011

	http.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New()

		time.Sleep(1 * time.Second)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Request handled.\nValue:" + id.String()))
	})

	fmt.Printf("Will launch server on port %d and tls on %d\n", port, tlsPort)

	go func() {
		err := http.ListenAndServe(":"+strconv.Itoa(port), nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		// generate a `Certificate` struct
		cert, _ := tls.LoadX509KeyPair("cert.crt", "cert.key")

		// create a custom server with `TLSConfig`
		s := &http.Server{
			Addr:    ":" + strconv.Itoa(tlsPort),
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
