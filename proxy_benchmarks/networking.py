from contextlib import closing, contextmanager
from pathlib import Path
from socket import AF_INET, SOCK_STREAM, socket
from subprocess import PIPE, Popen, run
from dataclasses import dataclass, field

from psutil import net_if_addrs
from subprocess import run
from tempfile import NamedTemporaryFile
from csv import DictReader
from io import StringIO


def is_socket_bound(port) -> bool:
    # Parse the currently active ports via tabular notation
    result = run(f"lsof -ti:{port}", shell=True, stdout=PIPE, stderr=PIPE)

    if result.stdout.decode().strip():
        return True
    else:
        return False


@contextmanager
def capture_network_traffic(output_path: Path | str, interface: str = "en0"):
    """
    :param interface: BSD network interface name

    https://knowledge.broadcom.com/external/article/171081/obtain-a-packet-capture-from-a-mac-compu.html

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
    # Additional logging content is written to stderr, so we don't needlessly flag an error here
    for output in [outputs.decode(), errors.decode()]:
        if output.strip():
            print("Network Outputs", output)


@dataclass
class PFConfig:
    scrub_anchors: list[str] = field(default_factory=list)
    nat_anchors: list[str] = field(default_factory=list)
    rdr_anchors: list[str] = field(default_factory=list)
    dummynet_anchors: list[str] = field(default_factory=list)
    anchors: list[str] = field(default_factory=list)
    load_anchors: list[str] = field(default_factory=list)

    def inject_file(self, content: str):
        """
        Inject file contents into the current PFConfig

        """
        # Do more specific searches first in the case of common prefixes
        field_mapping = sorted(
            self.field_mapping.items(),
            key=lambda x: len(x[0]),
            reverse=True,
        )

        for line in content.split("\n"):
            found_value = False

            # Comments don't have to be parsed
            if line.strip().startswith("#"):
                continue

            # Blank lines
            if not line.strip():
                continue

            for field_prefix, store in field_mapping:
                if line.startswith(field_prefix):
                    value = line.lstrip(field_prefix).strip()
                    store.append(value)
                    found_value = True

            if not found_value:
                raise ValueError(f"Unknown key in pf.conf: {line}")

    def to_string(self) -> str:
        """
        Creates a formatted PF configuration in the order required by pfctl.
        https://www.freebsd.org/cgi/man.cgi?query=pf.conf&sektion=5&manpath=freebsd-release-ports

        """
        content = ""

        for key, values in self.field_mapping.items():
            for value in values:
                content += f"{key} {value}\n"

        return content

    @property
    def field_mapping(self) -> dict[str, list[str]]:
        # Return in the order that is required in the pf.conf
        return {
            "scrub-anchor": self.scrub_anchors,
            "nat-anchor": self.nat_anchors,
            "rdr-anchor": self.rdr_anchors,
            "dummynet-anchor": self.dummynet_anchors,
            "anchor": self.anchors,
            "load anchor": self.load_anchors,
        }


@dataclass(frozen=True)
class SyntheticHostDefinition:
    name: str
    http_port: int | None = None
    https_port: int | None = None


class SyntheticHosts:
    def __init__(self, hosts: list[SyntheticHostDefinition]):
        self.hosts = hosts

    def configure(self) -> dict[SyntheticHostDefinition, str]:
        """
        Returns the synthetic IP for a given host file.

        Create additional 127 loopbacks, starting at index 2 - since 1 is already used by default

        """
        host_to_ip = {
            host: f"127.0.0.{i+2}"
            for i, host in enumerate(self.hosts)
        }

        for ip_address in host_to_ip.values():
            run(f"sudo ifconfig lo0 alias {ip_address} up", shell=True)

        custom_routing = []
        for host, ip_address in host_to_ip.items():
            if host.http_port:
                custom_routing.append(f"rdr pass on lo0 inet proto tcp from any to {ip_address} port 80 -> 127.0.0.1 port {host.http_port}")
            if host.https_port:
                custom_routing.append(f"rdr pass on lo0 inet proto tcp from any to {ip_address} port 443 -> 127.0.0.1 port {host.https_port}")

        with open("/etc/pf.conf") as file:
            default_routing = file.read()

        with NamedTemporaryFile("w+") as custom_rules_file:
            # The trailing newline is important or otherwise pfctl will be unable to read the file
            custom_rules_file.write("\n".join(custom_routing) + "\n")
            custom_rules_file.flush()
            custom_rules_file.seek(0)

            with NamedTemporaryFile("w+") as new_root_file:
                # Customize the pf config
                # Some more context on proper customization with some edge cases: https://github.com/basecamp/pow/issues/452
                pf_config = PFConfig(
                    rdr_anchors=['"proxy-benchmarks"'],
                    load_anchors=[f'"proxy-benchmarks" from "{custom_rules_file.name}"']
                )
                pf_config.inject_file(default_routing)

                # The trailing newline is important or otherwise pfctl will be unable to read the file
                new_root_file.write(pf_config.to_string() + "\n")
                new_root_file.flush()
                new_root_file.seek(0)

                run(f"sudo pfctl -e -f {new_root_file.name}", shell=True)

        return host_to_ip
