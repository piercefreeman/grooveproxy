import pytest

from proxy_benchmarks.cli.ssl_validity import execute_raw
from proxy_benchmarks.enums import MimicTypeEnum
from proxy_benchmarks.proxies.gomitmproxy import GoMitmProxy
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.proxies.martian import MartianProxy
from proxy_benchmarks.proxies.mitmproxy import MitmProxy
from proxy_benchmarks.proxies.node_http_proxy import NodeHttpProxy
from proxy_benchmarks.requests import ChromeRequest


@pytest.mark.parametrize(
    "proxy",
    [
        GoProxy(MimicTypeEnum.STANDARD),
        GoProxy(MimicTypeEnum.MIMIC),
        GoMitmProxy(MimicTypeEnum.STANDARD),
        GoMitmProxy(MimicTypeEnum.MIMIC),
        MartianProxy(),
        MitmProxy(),
        NodeHttpProxy(),
    ],
)
def test_ssl_validity(cli_object, proxy):
    request = ChromeRequest(headless=True, keep_open=False)

    execute_raw(
        cli_object,
        inspect_browser=False,
        request=request,
        proxies=[proxy],
    )
