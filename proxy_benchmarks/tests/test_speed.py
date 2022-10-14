from tempfile import TemporaryDirectory

import pytest

from proxy_benchmarks.cli.speed import execute_raw
from proxy_benchmarks.enums import MimicTypeEnum
from proxy_benchmarks.proxies.gomitmproxy import GoMitmProxy
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.proxies.martian import MartianProxy
from proxy_benchmarks.proxies.mitmproxy import MitmProxy
from proxy_benchmarks.proxies.node_http_proxy import NodeHttpProxy
from pathlib import Path


@pytest.mark.parametrize(
    "proxy",
    [
        GoProxy(MimicTypeEnum.STANDARD),
        GoProxy(MimicTypeEnum.MIMIC),
        GoMitmProxy(MimicTypeEnum.STANDARD),
        MartianProxy(),
        MitmProxy(),
        NodeHttpProxy(),
    ],
)
def test_speed_simple(cli_object, proxy):
    with TemporaryDirectory() as directory:
        directory = Path(directory)

        execute_raw(
            cli_object,
            data_path=directory,
            samples=5,
            proxies=[proxy],
        )

@pytest.mark.xfail(reason="crash because of http/2 protocol")
@pytest.mark.parametrize(
    "proxy",
    [
        GoMitmProxy(MimicTypeEnum.MIMIC),
    ]
)
def test_speed_simple_broken(cli_object, proxy):
    with TemporaryDirectory() as directory:
        directory = Path(directory)

        execute_raw(
            cli_object,
            data_path=directory,
            samples=5,
            proxies=[proxy],
        )
