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
from proxy_benchmarks.requests import ChromeRequestHeadfull
from functools import partial

# To test TCP connection, we need a valid https url
TEST_TCP_URL = "https://freeman.vc"

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

    with launch_proxy():
        chrome_runner = ChromeRequestHeadfull()

        fingerprints = get_fingerprint(
            TEST_TCP_URL,
            request_fn=partial(
                chrome_runner.handle_request,
                proxy="http://localhost:8080",
            )
        )

        # If multiple fingerprints came back for the domain (because of multiple outbound requests on different
        # TCP connections), ensure these are all the same. We expect the same request type
        # will have the same fingerprint over time so this is mostly a sanity check.
        fingerprint_digests = {record.digest for record in fingerprints}
        if len(fingerprint_digests) > 1:
            raise ValueError(f"Fingerprint values are not consistent across requests: `{fingerprint_digests}`.")

        print(fingerprint_digests)

        sleep(5)
