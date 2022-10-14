from tempfile import TemporaryDirectory

import pytest

from proxy_benchmarks.cli.load import execute_raw
from proxy_benchmarks.enums import MimicTypeEnum
from proxy_benchmarks.proxies.gomitmproxy import GoMitmProxy
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.proxies.martian import MartianProxy
from proxy_benchmarks.proxies.mitmproxy import MitmProxy
from proxy_benchmarks.proxies.node_http_proxy import NodeHttpProxy


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
def test_load_simple(cli_object, proxy):
    with TemporaryDirectory() as directory:
        execute_raw(
            cli_object,
            data_path=directory,
            runtime_seconds=5,
            proxies=[proxy],
        )