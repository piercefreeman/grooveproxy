from dataclasses import dataclass
from enum import Enum
from itertools import groupby
from pathlib import Path
from typing import Any

from dpkt.pcap import Reader as PcapReader
from dpkt.pcapng import Reader as PcapngReader
from ja3.ja3 import process_pcap

from proxy_benchmarks.enums import CipherEnum, TLSKnownExtensionEnum, TLSVersionEnum


@dataclass
class Ja3Record:
    """
    Simplified JA3 record.

    """
    raw: str
    digest: str


@dataclass
class HelloClientRecord:
    """
    Simplified HelloClient payload that carries the primary data

    """
    ip: dict[str, str]
    handshake: dict[str, str]


@dataclass
class Ja3RawPayload:
    """
    Data that is used as part of the Ja3 record.

    """
    tls_version: TLSVersionEnum
    ciphers: set[CipherEnum | str]
    extensions: set[TLSKnownExtensionEnum | int]
    elliptic_curves: set[TLSKnownExtensionEnum | str]
    elliptic_curve_formats: set[str]


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


class CaptureParser:
    def __init__(self, console):
        self.console = console

    def get_hello_client(self, capture, search_ip: str) -> HelloClientRecord:
        for packet in capture:
            layers = packet["_source"]["layers"]
            if "ip" not in layers:
                continue

            ip_definition = layers["ip"]

            if ip_definition["ip.dst"] != search_ip:
                continue
            if "tls" not in layers:
                continue
            if "tls.record" not in layers["tls"]:
                continue

            tls_records = (
                layers["tls"]["tls.record"]
                if isinstance(layers["tls"]["tls.record"], list)
                else [layers["tls"]["tls.record"]]
            )

            for record in tls_records:
                tls_handshake = record.get("tls.handshake", {})

                if tls_handshake:
                    self.console.print(f"Found TCP hello world", style="bold green")
                    # We want to get the first one that we can
                    return HelloClientRecord(
                        ip=ip_definition,
                        handshake=tls_handshake,
                    )

    def extract_extensions(self, record: HelloClientRecord) -> dict[TLSKnownExtensionEnum | int, dict[str, Any]]:
        """
        Given a hello client record, extract the extensions.

        Returns a mapping of {extension numerical type: payload}. See `TLSKnownExtensionEnum` for a list
        of known values, but there might be others now or in the future. As such we fallback to returning the
        extension type as an integer if it's not matched within our enum.

        """
        extensions = {
            payload_value["tls.handshake.extension.type"]: payload_value
            for payload_value in record.handshake.values()
            if "tls.handshake.extension.type" in payload_value
        }

        enum_mapping = {
            extension_type.value: extension_type
            for extension_type in TLSKnownExtensionEnum
        }

        # Where possible convert to enums
        return {
            enum_mapping.get(int(key), int(key)): value
            for key, value in extensions.items()
        }

    def build_ja3_payload(self, record: HelloClientRecord) -> Ja3RawPayload:
        """
        Record a payload of values that Ja3 is known to utilize, formatted in human readible terms.

        """
        cipher_mapping = {
            **{
                cipher.value: cipher
                for cipher in CipherEnum
            },
            # Add in additional GREASE values
            **{
                0xdada: "Reserved (GREASE)",
                0x1a1a: "Reserved (GREASE)",
                0x2a2a: "Reserved (GREASE)",
                0x3a3a: "Reserved (GREASE)",
                0x4a4a: "Reserved (GREASE)",
                0x5a5a: "Reserved (GREASE)",
                0x6a6a: "Reserved (GREASE)",
                0x7a7a: "Reserved (GREASE)",
                0x8a8a: "Reserved (GREASE)",
                0x9a9a: "Reserved (GREASE)",
                0xaaaa: "Reserved (GREASE)",
                0xbaba: "Reserved (GREASE)",
                0xcaca: "Reserved (GREASE)",
            }
        }

        tls_version = TLSVersionEnum(int(record.handshake["tls.handshake.version"], 16))
        ciphers = {
            cipher_mapping.get(int(cipher, 16), cipher)
            for cipher in record.handshake["tls.handshake.ciphersuites"]["tls.handshake.ciphersuite"]
        }
    
        extensions = self.extract_extensions(record)

        extension_types = set(extensions.keys())
        elliptic_curves = {
            cipher_mapping.get(int(curve, 16), curve)
            for curve in extensions[TLSKnownExtensionEnum.SUPPORTED_GROUPS]["tls.handshake.extensions_supported_groups"]["tls.handshake.extensions_supported_group"]
        }
        elliptic_curve_point_formats = set(extensions[TLSKnownExtensionEnum.EC_POINT_FORMATS]["tls.handshake.extensions_ec_point_formats"]["tls.handshake.extensions_ec_point_format"])

        return Ja3RawPayload(
            tls_version=tls_version,
            ciphers=ciphers,
            extensions=extension_types,
            elliptic_curves=elliptic_curves,
            elliptic_curve_formats=elliptic_curve_point_formats,
        )
