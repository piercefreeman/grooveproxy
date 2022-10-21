# Groove

Python APIs for Groove, a proxy server built for web crawling and unit test mocking. Highlights of its primary features:

- HTTP and HTTPs support over HTTP/1 and HTTP/2.
- Local CA certificate generation and installation on Mac and Linux to support system curl and Chromium.
- Different tiers of caching support - from disabling completely to aggressively maintaining all body archives.
- Limit outbound requests of the same URL to 1 concurrent request to save on bandwidth if requests are already inflight.
- Record and replay requests made to outgoing servers. Recreate testing flows in unit tests while separating them from crawling business logic.
- 3rd party proxy support for commercial proxies.
- Custom TLS Hello Client support to maintain a Chromium-like TLS handshake while intercepting requests and re-forwarding on packets.

For more information, see the [Github](https://github.com/piercefreeman/grooveproxy) project.

## Usage

Add groove to your project and install the local certificates that allow for https certificate generation:

```
pip install groove
install-ca
```

Instantiating Groove with the default parameters is usually fine for most deployments. To ensure we clean up resources once you're completed with the proxy, wrap your code in the `launch` contextmanager.

```python
from groove.proxy import Groove
from requests import get
from pathlib import Path

proxy = Groove()
with proxy.launch():
    response = get(
        "https://www.example.com",
        proxies={
            "http": proxy.base_url_proxy,
            "https": proxy.base_url_proxy,
        },
        verify=str(Path("~/.grooveproxy/ca.crt").expanduser()),
    )
    assert response.status_code == 200
```

Create a fully fake outbound for testing:

```python
from groove.proxy import Groove
from groove.tape import TapeRecord, TapeRequest, TapeResponse, TapeSession
from requests import get
from pathlib import Path

records = [
    TapeRecord(
        request=TapeRequest(
            url="https://example.com:443/",
            method="GET",
            headers={},
            body=b"",
        ),
        response=TapeResponse(
            status=200,
            headers={},
            body=b64encode("Test response".encode())
        ),
    )
]

proxy = Groove()
with proxy.launch():
    proxy.tape_load(
        TapeSession(
            records=records
        )
    )

    response = get(
        "https://www.example.com",
        proxies={
            "http": proxy.base_url_proxy,
            "https": proxy.base_url_proxy,
        },
        verify=str(Path("~/.grooveproxy/ca.crt").expanduser())
    )
    assert response.content == b"Test response"
```
