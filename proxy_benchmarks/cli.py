from pathlib import Path
from socket import gethostbyname
from subprocess import run
from tempfile import TemporaryDirectory
from time import sleep
from urllib.parse import urlparse

from click import command
from typing import Callable

from proxy_benchmarks.fingerprinting import ja3_by_ip, Ja3Record
from proxy_benchmarks.networking import capture_network_traffic
from proxy_benchmarks.proxies.mitmproxy import launch_proxy
from proxy_benchmarks.requests import ChromeRequest, PythonRequest
from functools import partial
from rich.console import Console

# To test TCP connection, we need a valid https url
TEST_TCP_URL = "https://freeman.vc"

console = Console()


def get_fingerprint(url: str, request_fn: Callable[[str], None]) -> dict[str, list[Ja3Record]]:
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


@command()
def main():
    # Ensure we have sudo permissions
    print("proxy-benchmarks needs to capture network traffic...")
    run(f"sudo echo 'Confirmation success...\n'", shell=True)

    runners = [
        PythonRequest(),
        ChromeRequest(headless=True),
        ChromeRequest(headless=False),
    ]

    with launch_proxy():
        print("Launched proxy...")

        for runner in runners:
            fingerprint_by_proxy = {}

            for proxy in ["http://localhost:8080", None]:
                divider = "-" * console.width
                console.print(f"{divider}\nTesting {runner} with proxy {proxy}\n{divider}", style="bold blue")

                fingerprints = get_fingerprint(
                    TEST_TCP_URL,
                    request_fn=partial(
                        runner.handle_request,
                        proxy=proxy,
                    )
                )

                # If multiple fingerprints came back for the domain (because of multiple outbound requests on different
                # TCP connections), ensure these are all the same. We expect the same request type
                # will have the same fingerprint over time so this is mostly a sanity check.
                fingerprint_digests = {record.digest for record in fingerprints}
                if len(fingerprint_digests) > 1:
                    console.print(f"Fingerprint values are not consistent across requests: `{fingerprint_digests}`.", style="bold red")
                    pass

                console.print(f"Results {runner} {proxy}: fingerprint_digests")
                fingerprint_by_proxy[proxy] = fingerprint_digests

            # Ensure the fingerprints are the same for the same request type, regardless of proxy
            fingerprint_values = {frozenset(digests) for digests in fingerprint_by_proxy.values()}
            if len(fingerprint_values) > 1:
                console.print(f"Fingerprint values are not consistent across proxies: `{fingerprint_values}`.", style="bold red")

        sleep(5)
