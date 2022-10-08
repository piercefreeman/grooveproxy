from dataclasses import dataclass
from itertools import groupby
from pathlib import Path

from dpkt.pcap import Reader as PcapReader
from dpkt.pcapng import Reader as PcapngReader
from ja3.ja3 import process_pcap


@dataclass
class Ja3Record:
    raw: str
    digest: str


def ja3_by_ip(capture_path: str | Path) -> dict[str, list[Ja3Record]]:
    """
    Compute a fingerprint of the TCP connection, given a pcap network packet
    capture file. Group results by IP addresses of outgoing requests.

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
