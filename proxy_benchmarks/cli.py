from contextlib import contextmanager
from dataclasses import dataclass
from enum import Enum
from itertools import groupby
from pathlib import Path
from socket import gethostbyname
from subprocess import PIPE, Popen, run
from tempfile import TemporaryDirectory
from time import sleep
from urllib.parse import urlparse

from click import command
from dpkt.pcapng import Reader as PcapngReader
from dpkt.pcap import Reader as PcapReader
from ja3.ja3 import process_pcap
from psutil import net_if_addrs
from requests import get

# To test TCP connection, we need a valid https url
TEST_TCP_URL = "https://freeman.vc"

@dataclass
class Ja3Record:
    raw: str
    digest: str

def ja3_for_by_ip(capture_path: str | Path) -> dict[str, list[Ja3Record]]:
    """
    Read pcap captured file and return IP records by domain.

    """
    with open(capture_path, "rb") as file:
        try:
            capture = PcapReader(file)
        except ValueError as pcap_error:
            try:
                file.seek(0)
                capture = PcapngReader(file)
            except ValueError as pcapng_error:
                raise ValueError(f"Not a valid pcap or pcapng file: `{pcap_error}` `{pcapng_error}`")
        output = process_pcap(capture, any_port=True)

    return {
        ip_address: [
            Ja3Record(raw=ja3_record["ja3"], digest=ja3_record["ja3_digest"])
            for ja3_record in ja3_records
        ]
        for ip_address, ja3_records in groupby(
            sorted(output, key=lambda record: record["destination_ip"]),
            key=lambda record: record["destination_ip"],
        )
    }


@contextmanager
def capture_network_traffic(output_path: Path | str, interface: str = "en0"):
    """
    :param interface: BSD network interface name

    """
    allowed_interfaces = list(net_if_addrs().keys())
    if interface not in allowed_interfaces:
        raise ValueError(f"Interface `{interface}` not found in allowed list `{allowed_interfaces}`.")

    output_path = Path(output_path).expanduser()
    process = Popen(f"sudo tcpdump -i {interface} -s 0 -B 524288 -w '{output_path}'", stdout=PIPE, stderr=PIPE, shell=True)

    yield

    # We need to kill with sudo permissions, so the `terminate` signal doesn't count
    run(f"sudo kill {process.pid}", shell=True)

    outputs, errors = process.communicate()
    print(outputs.decode())
    print(errors.decode())


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

        network_records = ja3_for_by_ip(capture_path)

    # We only care about the ones to our test domain, since other network requests
    # might have been made by the system daemons or user in this capture interval
    test_host = urlparse(TEST_TCP_URL)
    search_ip = gethostbyname(test_host.netloc) 

    test_records = network_records[search_ip]
    print(test_records)
