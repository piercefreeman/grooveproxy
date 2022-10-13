// https://github.com/google/martian/blob/master/proxy.go
// https://github.com/google/martian/blob/master/cmd/proxy/main.go
// Copyright 2015 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/google/martian"
	"github.com/google/martian/mitm"
)

func getCredentials(cert *string, key *string) (*x509.Certificate, interface{}, error) {
	var x509c *x509.Certificate
	var priv interface{}

	tlsc, err := tls.LoadX509KeyPair(*cert, *key)
	if err != nil {
		return nil, nil, err
	}
	priv = tlsc.PrivateKey

	x509c, err = x509.ParseCertificate(tlsc.Certificate[0])
	if err != nil {
		return nil, nil, err
	}

	return x509c, priv, nil
}

func main() {
	var (
		port         = flag.Int("port", 8080, "bind port")
		cert         = flag.String("cert", "ssl/ca.crt", "filepath to the CA certificate used to sign MITM certificates")
		key          = flag.String("key", "ssl/ca.key", "filepath to the private key of the CA used to sign MITM certificates")
		organization = flag.String("organization", "Martian Proxy", "organization name for MITM certificates")
		validity     = flag.Duration("validity", time.Hour, "window of time that MITM certificates are valid")
	)

	flag.Parse()

	p := martian.NewProxy()
	defer p.Close()

	l, err := net.Listen("tcp", ":"+strconv.Itoa(*port))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("martian: starting proxy on %s", l.Addr().String())

	x509c, priv, err := getCredentials(cert, key)
	if err != nil {
		log.Fatal(err)
	}

	mc, err := mitm.NewConfig(x509c, priv)
	if err != nil {
		log.Fatal(err)
	}

	mc.SetValidity(*validity)
	mc.SetOrganization(*organization)

	// Always require server-side TLS validation
	mc.SkipTLSVerify(false)

	p.SetMITM(mc)

	go p.Serve(l)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)

	<-sigc

	log.Println("martian: shutting down")
	os.Exit(0)
}
