// https://github.com/elazarl/goproxy/blob/a92cc753f88eb1d5f3ca49bd91da71fe815537ca/examples/goproxy-customca/cert.go
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"runtime"

	goproxy "github.com/piercefreeman/goproxy"
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

func getLocalCAPaths() (localPath string, localCAPath string, localCAKey string) {
	user, err := user.Current()
	if err != nil {
		log.Fatal(fmt.Errorf("Unable to resolve current user: %w", err))
	}

	localPath = path.Join(user.HomeDir, ".grooveproxy")
	localCAPath = path.Join(localPath, "ca.crt")
	localCAKey = path.Join(localPath, "ca.key")

	return localPath, localCAPath, localCAKey
}

func installCA() {
	/*
	 * Determine if a certificate has already been generated for the proxy and if not will create
	 * one in the user's home directory under `.grooveproxy/{ca.crt,ca.key}`.
	 */
	localPath, localCAPath, localCAKey := getLocalCAPaths()

	// Ensure this folder is created
	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	// Check for existing CA certificates
	if _, err := os.Stat(localCAPath); err == nil {
		log.Fatal(fmt.Errorf("CA certificate already generated, remove to regenerate:\n `rm %s && rm %s`\n", localCAPath, localCAKey))
	}

	cmd := exec.Command("openssl", "genrsa", "-out", "ca.key", "2048")
	cmd.Dir = localPath
	if _, err := cmd.Output(); err != nil {
		log.Fatal(err)
	}

	cmd = exec.Command("openssl", "req", "-new", "-x509", "-key", "ca.key", "-out", "ca.crt", "-subj", "/C=US/ST=CA/L= /O= /OU= /CN=GrooveProxy/emailAddress= ")
	cmd.Dir = localPath
	if _, err := cmd.Output(); err != nil {
		log.Fatal(err)
	}

	switch {
	case runtime.GOOS == "linux":
		installCALinux(localCAPath)
	case runtime.GOOS == "darwin":
		installCADarwin(localCAPath)
	default:
		log.Fatal("Unknown OS, can't perform local installation")
	}

	log.Println("Certificate generation completed.")
}

func installCALinux(caPath string) {
	// System installation path
	user, err := user.Current()
	if err != nil {
		log.Fatal(fmt.Errorf("Unable to resolve current user: %w", err))
	}

	systemPath := "/usr/local/share/ca-certificates/grooveproxy-ca.crt"
	cmd := exec.Command("sudo", "cp", caPath, systemPath)
	if _, err = cmd.Output(); err != nil {
		log.Fatal(err)
	}

	cmd = exec.Command("sudo", "update-ca-certificates")
	if _, err = cmd.Output(); err != nil {
		log.Fatal(err)
	}

	// Chrome / Chromium doesn't respect the system certificate store on Ubuntu
	// Instead use
	// https://chromium.googlesource.com/chromium/src/+/master/docs/linux/cert_management.md
	certUtilPath := fmt.Sprintf("sql:%s/.pki/nssdb", user.HomeDir)
	cmd = exec.Command("sudo", "certutil", "-d", certUtilPath, "-A", "-t", "C,,", "-n", "grooveproxy", "-i", systemPath)
	if _, err = cmd.Output(); err != nil {
		log.Fatal(err)
	}
}

func installCADarwin(caPath string) {
	cmd := exec.Command("sudo", "security", "add-trusted-cert", "-d", "-p", "ssl", "-p", "basic", "-k", "/Library/Keychains/System.keychain", caPath)
	if _, err := cmd.Output(); err != nil {
		log.Fatal(err)
	}
}
