import pytest

from proxy_benchmarks.cli.fingerprinting import compare_dynamic_raw
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
def test_fingerprint_independent(cli_object, proxy):
    """
    Ensure that we can benchmark each of the fingerprints against a baseline SSL connection
    """
    compare_dynamic_raw(
        cli_object,
        ChromeRequest(headless=True),
        [proxy]
    )


def test_fingerprint_multiple(cli_object):
    """
    Ensure that we can compare standard and mimic values with one another
    """
    compare_dynamic_raw(
        cli_object,
        ChromeRequest(headless=True),
        [
            GoProxy(MimicTypeEnum.STANDARD),
            GoProxy(MimicTypeEnum.MIMIC),
        ]
    )
