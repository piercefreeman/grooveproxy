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

Our proxies need additional setup when running locally. For convenience we have a setup script that handles this across all proxies. This script will install the necessary dependencies across node, go, and python to power the necessary proxies. It will also generate root certificates for the MITM proxy handlers. Expect to see multiple popups to trust these credentials.

```
./setup.sh
```

### mitmproxy

`mitmproxy` will automatically create a root certificate to authorize requests. To then trust this certificate locally, follow the instructions here: https://docs.mitmproxy.org/stable/concepts-certificates/

## Requests

### Playwright

Install the browsers that we want to test through the proxy.

```
poetry run playwright install chromium
```

## Benchmarking Harness

MITM proxies re-issue the commands that clients give them. Test whether fingerprints of the proxies align with their originally issuing browsers.

```
poetry run benchmark fingerprint execute [--output-directory ./fingerprint-capture]
```

View a more specific breakdown of the Ja3 fingerprint differences between the proxy and baseline (will execute the baseline comparison by default).

```
poetry run benchmark fingerprint compare-dynamic --proxy gomitmproxy
```

Conduct a load test of each proxy server, separately over http and https connections since https has additional overhead of having to manage the server->proxy certificate decryption and the proxy->client re-encryption.

```
poetry run benchmark load-test execute --data-path ./load-test
poetry run benchmark load-test analyze --data-path ./load-test
```

Conduct a speed test of the MITM host certificate generation process. In the wild we expect this to happen relatively frequently (every time we visit a new host) whereas in our load test this was completely excluded, because all proxies have a method to cache previously generated certificates either in memory or on disk.

```
poetry run benchmark speed-test execute --data-path ./speed-test
poetry run benchmark speed-test analyze --data-path ./speed-test
```

## Debugging

Q. I'm seeing an `ERR_CERT_AUTHORITY_INVALID` during tests.
A. Each OS (and potentially browser within that OS) has a different location where it stores certificates. On Ubuntu, for instance Chrome has its own credential storage manager [[1]](https://serverfault.com/questions/946756/ssl-certificate-in-system-store-not-trusted-by-chrome) [[2]](https://chromium.googlesource.com/chromium/src/+/master/docs/linux/cert_management.md).

Q. How do I perform a test inside of the docker image?
A.

```
docker-compose run -it benchmark test -k test_fingerprint_independent
```
