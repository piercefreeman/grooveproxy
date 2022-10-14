from tempfile import TemporaryDirectory

import pytest
from pathlib import Path

from proxy_benchmarks.cli.load import execute_raw, analyze_raw
from proxy_benchmarks.enums import MimicTypeEnum
from proxy_benchmarks.proxies.gomitmproxy import GoMitmProxy
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.proxies.martian import MartianProxy
from proxy_benchmarks.proxies.mitmproxy import MitmProxy
from proxy_benchmarks.proxies.node_http_proxy import NodeHttpProxy


@pytest.mark.load
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
        directory = Path(directory)

        # Execute the trials
        execute_raw(
            cli_object,
            output_path=directory,
            runtime_seconds=5,
            proxies=[None, proxy],
        )

        # Now analyze
        df = analyze_raw(directory, [None, proxy])

        # Ensure
        baseline_http_failure = df[(df.proxy == "baseline") & (df.protocol == "http")]["Failure Count"].iloc[0]
        baseline_https_failure = df[(df.proxy == "baseline") & (df.protocol == "https")]["Failure Count"].iloc[0]
        proxy_http_failure = df[(df.proxy == proxy.short_name) & (df.protocol == "http")]["Failure Count"].iloc[0]
        proxy_https_failure = df[(df.proxy == proxy.short_name) & (df.protocol == "https")]["Failure Count"].iloc[0]

        # Under this shallow load we shouldn't see any errors
        assert baseline_http_failure == "0"
        assert baseline_https_failure == "0"
        assert proxy_http_failure == "0"
        assert proxy_https_failure == "0"
