# Groove

Groove is an opinionated proxy server built for web crawling and unit test mocking. It's based on [goproxy](https://github.com/elazarl/goproxy), a well supported proxy implementation written in Go. It builds on this foundation to include:

- HTTP and HTTPs support over HTTP/1 and HTTP/2.
- Local CA certificate generation and installation on Mac and Linux to support system curl and Chromium.
- Different tiers of caching support. Standard caching will respect server-driven "Cache-Control" HTTP response headers. Aggressive caching will always cache a page regardless of headers. Disabling caching will always fetch new contents regardless of server preference.
- Limit outbound requests of the same URL to 1 concurrent request to save on bandwidth if requests are already inflight.
- Record and replay requests made to outgoing servers. Recreate testing flows in unit tests while separating them from crawling business logic.
- 3rd party proxy support for commercial proxies.
- Custom TLS Hello Client support to maintain a Chromium-like TLS handshake while intercepting requests and re-forwarding on packets.
- API clients for Python and Node.

## Proxy Benchmarks

Before settling on goproxy, we benchmarked a variety of MITM proxy servers across Python, Go, and Node. To view the benchmarking code and results see [proxy-benchmarks](./proxy-benchmarks/).
