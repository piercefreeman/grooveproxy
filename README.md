# proxy-benchmarks
Benchmark open web proxies

The mark of a good web proxy, which routes from local -> proxy -> remote:
1. Fast; minimal computational processing between receiving a request and forwarding it along
2. Concurrent; ability to handle multiple requests in parallel
3. Transparent; appearing as if requets came from the source

Transparency in reconstructing requests:
1. Maintain TLS Fingerprints from client. We use [ja3](https://github.com/salesforce/ja3) to check for fingerprint identity.

```
https://knowledge.broadcom.com/external/article/171081/obtain-a-packet-capture-from-a-mac-compu.html
```

# Proxies

## mitmproxy

Will be installed as part of the benchmarking executable. Futher installation steps are required:

1. Install https certificate for man-in-the-middle interception of https resources.

Details: https://docs.mitmproxy.org/stable/concepts-certificates/
