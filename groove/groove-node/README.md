# Groove

Node APIs for Groove, a proxy server built for web crawling and unit test mocking. Highlights of its primary features:

- HTTP and HTTPs support over HTTP/1 and HTTP/2.
- Local CA certificate generation and installation on Mac and Linux to support system curl and Chromium.
- Different tiers of caching support - from disabling completely to aggressively maintaining all body archives.
- Limit outbound requests of the same URL to 1 concurrent request to save on bandwidth if requests are already inflight.
- Record and replay requests made to outgoing servers. Recreate testing flows in unit tests while separating them from crawling business logic.
- 3rd party proxy support for commercial proxies.
- Custom TLS Hello Client support to maintain a Chromium-like TLS handshake while intercepting requests and re-forwarding on packets.

For more information, see the [Github](https://github.com/piercefreeman/grooveproxy) project.

## Usage

Add groove to your project and generate the local certificates.

```
npm install @piercefreeman/groove
npx @piercefreeman/groove install-ca
```

```javascript
import { Grove } from '@piercefreeman/groove'
import { TapeSession } from '@piercefreeman/groove/tape'
import { fetchWithProxy } from '@piercefreeman/groove/utilities'

const main = async () => {
    const proxy = new Groove(
        commandTimeout?;
        port?;
        controlPort?;
        proxyServer?;
        proxyUsername?;
        proxyPassword?:;
    )
    await proxy.launch()

    const mockedSession = new TapeSession(
        [
            {
                request: {
                    url: "https://example.com:443/",
                    method: "GET",
                    headers: {},
                    body: Buffer.from(""),
                },
                response: {
                    status: 200,
                    headers: {},
                    body: Buffer.from("Test response")
                }
            }
        ]
    )

    await proxy.tapeLoad(mockedSession);

    const response = await fetchWithProxy("https://example.com", proxy);
    console.log(response) // "Test response"
}
```
