# proxy-benchmarks
Benchmark open web proxies

The mark of a good web proxy, which routes from local -> proxy -> remote:
1. Fast; minimal computational processing between receiving a request and forwarding it along
2. Concurrent; ability to handle multiple requests in parallel
3. Transparent; appearing as if requets came from the source

Transparency in reconstructing requests:
1. Maintain TLS Fingerprints from client

```
https://knowledge.broadcom.com/external/article/171081/obtain-a-packet-capture-from-a-mac-compu.html
```
