# proxy-benchmarks
Benchmark open man-in-the-middle proxies, for use in web crawling and unit test construction.

## Background

This benchmarking is geared to find a proxy application for use in web crawl caching and test harnesses. As such, we need the following functionality:

- HTTP + HTTP(s) support. HTTP/2 is optional
- Client overrides of defined webpages, so we are able to return a mocked version of the page at test time
- Capture client requests for later use in tests. I sometimes refer to this as "replaying the tape," assuming that a cassette started rolling at request time.

Based on these requirements, especially the https support, we can't leverage normal forward proxies. Standard proxies rely on issuing a TCP handshake between the client and upstream server via http connect tunneling, so the proxy server doesn't have access to the request or response content. This has the benefit of ensuring the security of client connections. It also has the positive upshot of providing "transparency" to the proxy, so if destination servers rely on fingerprinting the TCP handshake of the client, it will appear as if the requests are coming from a valid origin.

The mark of a good mitm proxy, which routes from local -> proxy -> remote:
1. Fast; minimal computational processing between receiving a request and forwarding it along
2. Concurrent; ability to handle multiple requests in parallel
3. Transparent; appearing as if requets came from the source

Transparency in reconstructing requests:
1. Maintain TLS Fingerprints from client. We use [ja3](https://github.com/salesforce/ja3) to check for fingerprint identity.

Tested on OS X with Python 3.10.

## Proxies

Our proxies need additional setup when running locally.

### mitmproxy

`mitmproxy` will automatically create a root certificate to authorize requests. To then trust this certificate locally, follow the instructions here: https://docs.mitmproxy.org/stable/concepts-certificates/

### node-http-proxy

First, install the node requirements.

```
cd proxy_benchmarks/assets/node_http_proxy && npm install
```

Install and trust the root certificate which is used to create synthetic certificates for each new host conducted in the test.

```
cd proxy_benchmarks/assets/node_http_proxy && npm run setup
```

### gomitmproxy

Install the executable dependencies and setup the ssh credentials.

```
cd proxy_benchmarks/assets/gomitmproxy
go install
./setup.sh
```

### martian

Install the executable dependencies and setup the ssh credentials.

```
cd proxy_benchmarks/assets/martian
go install
./setup.sh
```

## Requests

### Playwright

Install the browsers that we want to test through the proxy.

```
poetry run playwright install chromium
```
