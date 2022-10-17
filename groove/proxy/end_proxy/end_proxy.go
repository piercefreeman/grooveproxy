package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"

	goproxy "github.com/piercefreeman/goproxy"
	"github.com/piercefreeman/goproxy/ext/auth"
)

/*
 * Simple implementation of an end proxy used to route
 * to the Internet. Used for testing purposes to represent
 * a third party proxy.
 */
func main() {
	username, password := "foo", "bar"

	// start end proxy server
	endProxy := goproxy.NewProxyHttpServer()
	endProxy.Verbose = true
	auth.ProxyBasic(endProxy, "my_realm", func(user, pwd string) bool {
		return user == username && password == pwd
	})
	log.Println("serving end proxy server at localhost:8082")

	go func() {
		go http.ListenAndServe("localhost:8082", endProxy)
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)

	<-sigc

	log.Println("groove-end-proxy: shutting down")
	os.Exit(0)
}
