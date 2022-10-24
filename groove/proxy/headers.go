package main

import (
	"net/http"

	goproxy "github.com/piercefreeman/goproxy"
)

type HeaderDefinition struct {
	tapeID       string
	resourceType string
}

// Don't prefix with `Prefix` - chromium appears to have specific manipulation
// routines when a header is prefixed with `Proxy-`
const (
	ProxyResourceType   = "Resource-Type"
	ProxyTapeIdentifier = "Tape-ID"
)

func setupHeadersMiddleware(proxy *goproxy.ProxyHttpServer) {
	/*
	 * This should be mounted before other dependent middlewares
	 */
	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			ctx.UserData = &HeaderDefinition{
				tapeID:       r.Header.Get(ProxyTapeIdentifier),
				resourceType: r.Header.Get(ProxyResourceType),
			}

			// Remove the extracted keys so they're not passed on
			r.Header.Del(ProxyTapeIdentifier)

			// Currently ProxyResourceType is also consumed directly by the dialer, which doesn't
			// have access to the larger context. Keep it redundant for now.
			//r.Header.Del(ProxyResourceType)

			return r, nil
		},
	)
}
