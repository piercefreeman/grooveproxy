// https://github.com/elazarl/goproxy/blob/a92cc753f88eb1d5f3ca49bd91da71fe815537ca/examples/goproxy-customca/cert.go
package main

import (
	"crypto/tls"
	"crypto/x509"

	"github.com/elazarl/goproxy"
)

func setCA(caCert string, caKey string) error {
	// Override the default support: https://github.com/elazarl/goproxy/blob/fbd10ff4f5a16de73dca5030fc12245548f76141/https.go#L32
	goproxyCa, err := tls.LoadX509KeyPair(caCert, caKey)
	if err != nil {
		return err
	}
	if goproxyCa.Leaf, err = x509.ParseCertificate(goproxyCa.Certificate[0]); err != nil {
		return err
	}
	goproxy.GoproxyCa = goproxyCa
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	return nil
}
