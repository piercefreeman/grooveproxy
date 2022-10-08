from dataclasses import dataclass
from itertools import groupby
from pathlib import Path
from socket import gethostbyname
from subprocess import run
from tempfile import TemporaryDirectory
from time import sleep
from urllib.parse import urlparse

from click import command
from requests import get

from proxy_benchmarks.fingerprinting import ja3_by_ip
from proxy_benchmarks.networking import capture_network_traffic
from proxy_benchmarks.proxies.mitmproxy import launch_proxy

# To test TCP connection, we need a valid https url
TEST_TCP_URL = "https://freeman.vc"


@command()
def main():
    # Ensure we have sudo permissions
    print("proxy-benchmarks needs to capture network traffic...")
    run(f"sudo echo 'Confirmation success...\n'", shell=True)

    with TemporaryDirectory() as directory:
        capture_path = Path(directory) / "capture.pcap"
        with capture_network_traffic(capture_path):
            print("Capturing traffic...")
            response = get(TEST_TCP_URL)
            assert response.ok

            # We notice an occasional lag in writing capture details to disk, sleep to allow
            # the process to catch up
            sleep(2)

        network_records = ja3_by_ip(capture_path)

    # We only care about the ones to our test domain, since other network requests
    # might have been made by the system daemons or user in this capture interval
    test_host = urlparse(TEST_TCP_URL)
    search_ip = gethostbyname(test_host.netloc) 

    test_records = network_records[search_ip]
    print(test_records)

    process = launch_proxy()
    sleep(2)
    get(
        "https://freeman.vc",
        proxies={"https": "http://localhost:8080"},
        verify=False,
    )
    sleep(5)
    process.terminate()
