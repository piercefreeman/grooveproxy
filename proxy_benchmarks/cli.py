from csv import DictReader
from dataclasses import asdict
from functools import partial
from json import dump, load
from pathlib import Path
from socket import gethostbyname
from subprocess import run
from tempfile import TemporaryDirectory
from time import sleep
from typing import Callable
from urllib.parse import urlparse

import pandas as pd
from click import Path as ClickPath
from click import group, option
from rich.console import Console

from proxy_benchmarks.fingerprinting import Ja3Record, ja3_by_ip
from proxy_benchmarks.load_test import LoadTestResults, run_load_test
from proxy_benchmarks.networking import capture_network_traffic
from proxy_benchmarks.proxies.base import ProxyBase
from proxy_benchmarks.proxies.gomitmproxy import GoMitmProxy
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.proxies.martian import MartianProxy
from proxy_benchmarks.proxies.mitmproxy import MitmProxy
from proxy_benchmarks.proxies.node_http_proxy import NodeHttpProxy
from proxy_benchmarks.requests import ChromeRequest, PythonRequest, RequestBase

# To test TCP connection, we need a valid https url
TEST_TCP_URL = "https://freeman.vc"

console = Console(soft_wrap=True)


def get_fingerprint(url: str, request_fn: Callable[[str], None]) -> dict[str, list[Ja3Record]]:
    """
    Given a URL to test, and a function which issues a network request, return a dictionary
    of IP addresses to a list of JA3 fingerprints.

    """
    with TemporaryDirectory() as directory:
        capture_path = Path(directory) / "capture.pcap"
        with capture_network_traffic(capture_path):
            print("Capturing traffic...")
            request_fn(url)

            # We notice an occasional lag in writing capture details to disk, sleep to allow
            # the process to catch up
            sleep(2)

        network_records = ja3_by_ip(capture_path)

    # We only care about the ones to our test domain, since other network requests
    # might have been made by the system daemons or user in this capture interval
    test_host = urlparse(url)
    search_ip = gethostbyname(test_host.netloc) 

    return network_records[search_ip]


@group()
def main():
    pass

@main.command()
def fingerprint(self):
    # Ensure we have sudo permissions
    print("proxy-benchmarks needs to capture network traffic...")
    run(f"sudo echo 'Confirmation success...\n'", shell=True)

    proxies: list[ProxyBase] = [
        MitmProxy(),
        NodeHttpProxy(),
        GoMitmProxy(),
        MartianProxy(),
        GoProxy(),
    ]

    runners: list[RequestBase] = [
        PythonRequest(),
        ChromeRequest(headless=True),
        ChromeRequest(headless=False),
    ]

    for proxy in proxies:
        print(f"{proxy}: Launched proxy...")

        for runner in runners:
            fingerprint_by_proxy = {}

            # Some proxies have caching built-in, so we should disable this during testing to make
            # sure we start with a clean slate for later tests
            with proxy.launch():
                # Compare fingerprint signatures with and without proxy
                for proxy_url in [f"http://localhost:{proxy.port}", None]:
                    divider = "-" * console.width
                    console.print(f"{divider}\nTesting {runner} with proxy {proxy} ({proxy_url})\n{divider}", style="bold blue")

                    fingerprints = get_fingerprint(
                        TEST_TCP_URL,
                        request_fn=partial(
                            runner.handle_request,
                            proxy=proxy_url,
                        )
                    )

                    # If multiple fingerprints came back for the domain (because of multiple outbound requests on different
                    # TCP connections), ensure these are all the same. We expect the same request type
                    # will have the same fingerprint over time so this is mostly a sanity check.
                    fingerprint_digests = {record.digest for record in fingerprints}
                    if len(fingerprint_digests) > 1:
                        console.print(f"Fingerprint values are not consistent across requests: `{fingerprint_digests}`.", style="bold red")
                        pass

                    console.print(f"Results {runner} {proxy} ({proxy_url}): {fingerprint_digests}")
                    fingerprint_by_proxy[proxy_url] = fingerprint_digests

                # Ensure the fingerprints are the same for the same request type, regardless of proxy
                fingerprint_values = {frozenset(digests) for digests in fingerprint_by_proxy.values()}
                if len(fingerprint_values) > 1:
                    console.print(f"\nFingerprint values are not consistent across proxies:", style="bold red")
                    for proxy_url, digests in fingerprint_by_proxy.items():
                        console.print(f"  {proxy} {proxy_url}: {digests}")


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


@main.group()
def load_test():
    pass


@load_test.command()
@option("--data-path", type=ClickPath(dir_okay=True, file_okay=False), required=True)
def execute(data_path):
    output_path = Path(data_path).expanduser()
    output_path.mkdir(exist_ok=True)

    divider = "-" * console.width

    console.print(f"{divider}\nWill perform baseline load test...\n{divider}", style="bold blue")
    http_baseline_results = run_load_test("http_baseline_locust.conf", run_time_seconds=10)
    https_baseline_results = run_load_test("https_baseline_locust.conf", run_time_seconds=10)
    console.print("Baseline test completed...")

    finalize_results(output_path, "baseline", http_baseline_results, https_baseline_results)

    proxies: list[ProxyBase] = [
        MitmProxy(),
        #NodeHttpProxy(),
        #GoMitmProxy(),
        #MartianProxy(),
        #GoProxy(),
    ]

    for proxy in proxies:
        console.print(f"{divider}\nWill perform http load test with {proxy}...\n{divider}", style="bold blue")

        with proxy.launch():
            run_load_test(f"http_locust.conf", proxy_port=proxy.port, run_time_seconds=10)
            run_load_test(f"https_locust.conf", proxy_port=proxy.port, run_time_seconds=10)

            # Move somewhere permanent since these will be overridden
            finalize_results(output_path, proxy.short_name, http_baseline_results, https_baseline_results)

        console.print(f"Load test with {proxy} completed...")


@load_test.command()
@option("--data-path", type=ClickPath(exists=True, dir_okay=True, file_okay=False), required=True)
def analyze(data_path):
    data_path = Path(data_path).expanduser()

    proxies: list[ProxyBase] = [
        MitmProxy(),
        #NodeHttpProxy(),
        #GoMitmProxy(),
        #MartianProxy(),
        #GoProxy(),
    ]

    dataframes = []

    for proxy in proxies:
        for protocol in ["http", "https"]:
            with open(data_path / f"{proxy.short_name}_{protocol}.json") as file:
                payload = load(file)
                dataframes.append(
                    pd.DataFrame(payload["stats"]).assign(
                        proxy=proxy.short_name,
                        protocol=protocol,
                    )
                )

    df = pd.concat(dataframes)
    print(df)
