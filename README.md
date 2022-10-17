# Groove

Groove is an opinionated proxy server built for web crawling and unit test mocking. It's based on [goproxy](https://github.com/elazarl/goproxy), a well supported proxy implementation in go. It builds on this base to include:

- HTTP and HTTPs support over
- Certificate validation
- Different tiers of caching support depending on use-cases. Limit outbound requests of the same URL to 1 concurrent request to save on bandwidth.
- Record and replay requests made to outgoing servers. Recreate testing flows in unit tests while separating them from crawling business logic.
- 3rd party proxy support for.
- API clients in Python and Node.

## Proxy Benchmarks

Before settling on goproxy, we benchmarked a variety of MITM proxy servers across Python, Go, and Node.
