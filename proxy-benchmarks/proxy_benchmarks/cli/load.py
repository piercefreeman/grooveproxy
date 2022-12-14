from csv import DictReader
from dataclasses import asdict
from json import dump, load
from pathlib import Path

import pandas as pd
from click import (
    Path as ClickPath,
    group,
    option,
    pass_obj,
)

from proxy_benchmarks.enums import MimicTypeEnum
from proxy_benchmarks.load_test import LoadTestResults, run_load_server, run_load_test
from proxy_benchmarks.networking import SyntheticHostDefinition, SyntheticHosts
from proxy_benchmarks.proxies.base import ProxyBase
from proxy_benchmarks.proxies.gomitmproxy import GoMitmProxy
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.proxies.martian import MartianProxy
from proxy_benchmarks.proxies.mitmproxy import MitmProxy
from proxy_benchmarks.proxies.node_http_proxy import NodeHttpProxy


@group()
def load_test():
    pass


@load_test.command()
@option("--data-path", type=ClickPath(dir_okay=True, file_okay=False), required=True)
@option("--runtime-seconds", type=int, default=60)
@option("--proxy", type=str, multiple=True)
@pass_obj
def execute(obj, data_path, runtime_seconds, proxy: list[str]):
    output_path = Path(data_path).expanduser()
    output_path.mkdir(exist_ok=True)

    # TODO: Create mapping for CLI argument based on short name and global mapping

    proxies: list[ProxyBase] = [
        None,
        GoMitmProxy(MimicTypeEnum.STANDARD),
        GoMitmProxy(MimicTypeEnum.MIMIC),
        MitmProxy(),
        NodeHttpProxy(),
        MartianProxy(),
        GoProxy(MimicTypeEnum.STANDARD),
        GoProxy(MimicTypeEnum.MIMIC),
    ]

    execute_raw(obj, output_path, runtime_seconds, proxies)


@load_test.command()
@option("--data-path", type=ClickPath(exists=True, dir_okay=True, file_okay=False), required=True)
@option("--output-filename", type=str, default="results_load_test.csv")
def analyze(data_path, output_filename):
    output_filename = Path(output_filename).expanduser()
    data_path = Path(data_path).expanduser()

    proxies: list[ProxyBase] = [
        None,
        MitmProxy(),
        NodeHttpProxy(),
        GoMitmProxy(MimicTypeEnum.STANDARD),
        GoMitmProxy(MimicTypeEnum.MIMIC),
        MartianProxy(),
        GoProxy(MimicTypeEnum.STANDARD),
        GoProxy(MimicTypeEnum.MIMIC),
    ]

    df = analyze_raw(data_path, proxies)
    df.to_csv(output_filename)


def execute_raw(obj, output_path: Path, runtime_seconds: int, proxies: list[ProxyBase]):
    console = obj["console"]
    divider = obj["divider"]

    load_http_port = 3010
    load_https_port = 3011

    # Launch a synthetic host for our fake server because some MITM implementations
    # (specifically [gomitmproxy](https://github.com/AdguardTeam/gomitmproxy/blob/c7fb1711772b738d3f89b55615b6ac0c8e312367/proxy.go#L661)
    # but there might be others) require a tls connection on port 443.
    synthetic_ip_addresses = SyntheticHosts(
        [
            SyntheticHostDefinition(
                name="load-server",
                http_port=load_http_port,
                https_port=load_https_port,
            )
        ]
    ).configure()
    synthetic_ip_address = next(iter(synthetic_ip_addresses.values()))

    for proxy in proxies:
        # Special case for the baseline, since no proxies are used
        if proxy is None:
             with run_load_server(port=load_http_port, tls_port=load_https_port):
                console.print(f"{divider}\nWill perform baseline load test...\n{divider}", style="bold blue")
                http_baseline_results = run_load_test(f"http://{synthetic_ip_address}", "http_baseline_locust.conf", run_time_seconds=runtime_seconds)
                https_baseline_results = run_load_test(f"https://{synthetic_ip_address}", "https_baseline_locust.conf", run_time_seconds=runtime_seconds)
                console.print("Baseline test completed...")

                finalize_results(output_path, "baseline", http_baseline_results, https_baseline_results)
        else:
            # Restart the server every time to make sure that we're not leaking resources
            # that might affect the load times / testing
            with run_load_server(port=load_http_port, tls_port=load_https_port):
                with proxy.launch():
                    console.print(f"{divider}\nWill perform http load test with {proxy}...\n{divider}", style="bold blue")
                    http_baseline_results = run_load_test(f"http://{synthetic_ip_address}", f"http_locust.conf", proxy=proxy, run_time_seconds=runtime_seconds)

                    console.print(f"{divider}\nWill perform https load test with {proxy}...\n{divider}", style="bold blue")
                    https_baseline_results = run_load_test(f"https://{synthetic_ip_address}", f"https_locust.conf", proxy=proxy, run_time_seconds=runtime_seconds)

                    # Move somewhere permanent since these will be overridden
                    finalize_results(output_path, proxy.short_name, http_baseline_results, https_baseline_results)

                console.print(f"Load test with {proxy} completed...")


def analyze_raw(data_path: Path | str, proxies: list[ProxyBase]) -> pd.DataFrame:
    dataframes = []

    for proxy in proxies:
        short_name = proxy.short_name if proxy else "baseline"

        for protocol in ["http", "https"]:
            with open(data_path / f"{short_name}_{protocol}.json") as file:
                payload = load(file)
                dataframes.append(
                    pd.DataFrame(payload["stats"]).assign(
                        proxy=short_name,
                        protocol=protocol,
                    )
                )

    df = pd.concat(dataframes)
    df = df[df["Name"] == "/handle"]

    # Re-arrange the columns for easier readibility
    columns = ["proxy", "protocol"] + [col for col in df.columns if col not in ["proxy", "protocol"]]
    df = df[columns]

    return df


def finalize_results(
    output_path: Path,
    proxy_name: str,
    http_results: LoadTestResults,
    https_results: LoadTestResults,
):
    """
    Consolidates the raw results from Locust into one main json file and saves to the
    output path with a filename convention.

    """
    for prefix, results in [("http", http_results), ("https", https_results)]:
        consolidated_results = {}

        for key, path in asdict(results).items():
            with open(path) as file:
                contents = DictReader(file)
                consolidated_results[key] = list(contents)

        with open(output_path / f"{proxy_name}_{prefix}.json", "w") as file:
            dump(consolidated_results, file)
