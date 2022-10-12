from csv import DictReader
from dataclasses import asdict
from functools import partial
from json import dump, load
from pathlib import Path
from socket import gethostbyname
from subprocess import run
from tempfile import TemporaryDirectory
from time import sleep, time
from typing import Callable
from urllib.parse import urlparse

import pandas as pd
from click import Path as ClickPath
from click import group, option
from rich.console import Console

from proxy_benchmarks.fingerprinting import Ja3Record, ja3_by_ip
from proxy_benchmarks.load_test import LoadTestResults, run_load_test, run_load_server
from proxy_benchmarks.networking import capture_network_traffic, SyntheticHosts, SyntheticHostDefinition
from proxy_benchmarks.proxies.base import ProxyBase
from proxy_benchmarks.proxies.gomitmproxy import GoMitmProxy
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.proxies.martian import MartianProxy
from proxy_benchmarks.proxies.mitmproxy import MitmProxy
from proxy_benchmarks.proxies.node_http_proxy import NodeHttpProxy
from proxy_benchmarks.requests import ChromeRequest, PythonRequest, RequestBase
from requests import get
from tqdm import tqdm


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
def basic_ssl_test():
    divider = "-" * console.width

    proxies: list[ProxyBase] = [
        #MitmProxy(),
        NodeHttpProxy(),
        #GoMitmProxy(),
        #MartianProxy(),
        #GoProxy(),
    ]

    request = ChromeRequest(headless=False, keep_open=True)

    with run_load_server() as load_server_definition:
        synthetic_ip_addresses = SyntheticHosts(
            [
                SyntheticHostDefinition(
                    name="load-server",
                    http_port=load_server_definition["http"],
                    https_port=load_server_definition["https"],
                )
            ]
        ).configure()
        synthetic_ip_address = next(iter(synthetic_ip_addresses.values()))
        print("\nSynthetic IP", synthetic_ip_address)

        print("Waiting for manual client access...")
        if input(" > Press enter when ready...") != "":
            return

        for proxy in proxies:
                with proxy.launch():
                    console.print(f"{divider}\nTesting {request} with proxy {proxy})\n{divider}", style="bold blue")
                    request.handle_request(
                        f"https://{synthetic_ip_address}/handle",
                        proxy=f"http://localhost:{proxy.port}",
                    )

@main.command()
def fingerprint():
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
@option("--runtime-seconds", type=int, default=60)
def execute(data_path, runtime_seconds):
    output_path = Path(data_path).expanduser()
    output_path.mkdir(exist_ok=True)

    divider = "-" * console.width

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

    with run_load_server(port=load_http_port, tls_port=load_https_port) as load_server_definition:
        console.print(f"{divider}\nWill perform baseline load test...\n{divider}", style="bold blue")
        http_baseline_results = run_load_test(f"http://{synthetic_ip_address}", "http_baseline_locust.conf", run_time_seconds=runtime_seconds)
        https_baseline_results = run_load_test(f"https://{synthetic_ip_address}", "https_baseline_locust.conf", run_time_seconds=runtime_seconds)
        console.print("Baseline test completed...")

        finalize_results(output_path, "baseline", http_baseline_results, https_baseline_results)

    proxies: list[ProxyBase] = [
        GoMitmProxy(),
        MitmProxy(),
        NodeHttpProxy(),
        MartianProxy(),
        GoProxy(),
    ]

    for proxy in proxies:
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


@load_test.command()
@option("--data-path", type=ClickPath(exists=True, dir_okay=True, file_okay=False), required=True)
def analyze(data_path):
    data_path = Path(data_path).expanduser()

    proxies: list[ProxyBase] = [
        None,
        MitmProxy(),
        NodeHttpProxy(),
        GoMitmProxy(),
        MartianProxy(),
        GoProxy(),
    ]

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

    # TODO: perform aggregation in python
    df.to_csv("results.csv")


@main.command()
@option("--samples", type=int, default=100)
def speed_certificate_generation_test(samples):
    proxies: list[ProxyBase] = [
        MitmProxy(),
        NodeHttpProxy(),
        GoMitmProxy(),
        MartianProxy(),
        GoProxy(),
    ]

    proxy_samples = []

    with run_load_server() as load_server_definition:
        synthetic_ip_addresses = SyntheticHosts(
            [
                SyntheticHostDefinition(
                    name="load-server",
                    http_port=load_server_definition["http"],
                    https_port=load_server_definition["https"],
                )
            ]
        ).configure()
        synthetic_ip_address = next(iter(synthetic_ip_addresses.values()))

        # Clear out any previously generated certificates by opening and then closing
        # the context manager
        for proxy in proxies:
            with proxy.launch():
                pass

        proxy_definition = {
            "http": f"http://localhost:{proxy.port}",
            "https": f"http://localhost:{proxy.port}",
        }

        for proxy in proxies:
            for _ in tqdm(range(samples)):
                with proxy.launch():
                    start_time = time()
                    cold_start_response = get(
                        f"https://{synthetic_ip_address}",
                        proxies=proxy_definition,
                        verify=proxy.certificate_authority.public,
                    )
                    cold_start_time = time() - start_time

                    start_time = time()
                    warm_start_response = get(
                        f"https://{synthetic_ip_address}",
                        proxies=proxy_definition,
                        verify=proxy.certificate_authority.public,
                    )
                    warm_start_time = time() - start_time

                    proxy_samples.append(
                        proxy=proxy.short_name,
                        cold_start=cold_start_time,
                        cold_start_status=cold_start_response.status_code,
                        warm_start=warm_start_time,
                        warm_start_status=warm_start_response.status_code,
                    )

    with open("results_certificate_speed.csv", "w") as file:
        dump(proxy_samples, file)
