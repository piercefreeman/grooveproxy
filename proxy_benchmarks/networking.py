from contextlib import contextmanager
from dataclasses import dataclass, field
from pathlib import Path
from subprocess import PIPE, Popen, run
from tempfile import NamedTemporaryFile

from psutil import net_if_addrs

from proxy_benchmarks.io import is_docker, wrap_command_with_sudo
from proxy_benchmarks.process import terminate_all


def is_socket_bound(port) -> bool:
    # Parse the currently active ports via tabular notation
    result = run(["lsof", f"-ti:{port}"], stdout=PIPE, stderr=PIPE)

    if result.stdout.decode().strip():
        return True
    else:
        return False


@contextmanager
def capture_network_traffic(output_path: Path | str, interface: str | None = None):
    """
    :param interface: BSD network interface name

    https://knowledge.broadcom.com/external/article/171081/obtain-a-packet-capture-from-a-mac-compu.html

    """
    if interface is None:
        # Attempt to intelligently find the right interface
        if is_docker():
            interface = "eth0"
        else:
            interface = "en0"

    allowed_interfaces = list(net_if_addrs().keys())
    if interface not in allowed_interfaces:
        raise ValueError(f"Interface `{interface}` not found in allowed list `{allowed_interfaces}`.")

    output_path = Path(output_path).expanduser()
    print("Will capture traffic...")
    process = Popen(["tcpdump", "-i", interface, "-s", "0", "-B", "524288", "-w", output_path], stdout=PIPE, stderr=PIPE)

    yield

    # Determine if the process crashed before we've explicitly closed it
    if process.returncode is not None:
        raise ValueError(f"Process error: {process.stdout.read().decode()} {process.stderr.read().decode()}")

    print("Close capture...")
    terminate_all(process)
    print("Did close capture")

    outputs, errors = process.communicate()
    print("-- Network Capture Outputs --")
    # Additional logging content is written to stderr, so we don't needlessly flag an error here
    for output in [outputs.decode(), errors.decode()]:
        if output.strip():
            print(output)
    print("-- End Network Capture Outputs --")


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

        if is_docker():
            self.configure_linux(host_to_ip)
        else:
            self.configure_mac(host_to_ip)

        return host_to_ip


    def configure_linux(self, host_to_ip: dict[SyntheticHostDefinition, str]):
        for ip_address in host_to_ip.values():
            #run(wrap_command_with_sudo(["ip", "lo0", "alias", ip_address, "up"]))
            # dev (device): lo (ie. loopback)
            run(wrap_command_with_sudo(["ip", "addr", "add", ip_address, "dev", "lo"]))

        custom_routing = []
        for host, ip_address in host_to_ip.items():
            if host.http_port:
                #iptables -t nat -A PREROUTING -p tcp -d {ip_address} --dport 80 -j REDIRECT --to-port {host.http_port}
                #iptables -t nat -A OUTPUT -p tcp -d {ip_address} --dport 80 -j REDIRECT --to-port {host.http_port}
                run(wrap_command_with_sudo(["iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "-d", ip_address, "--dport", "80", "-j", "REDIRECT", "--to-port", str(host.http_port)]))
                run(wrap_command_with_sudo(["iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "-d", ip_address, "--dport", "80", "-j", "REDIRECT", "--to-port", str(host.http_port)]))
            if host.https_port:
                #iptables -t nat -A PREROUTING -p tcp -d {ip_address} --dport 443 -j REDIRECT --to-port {host.http_port}
                #iptables -t nat -A OUTPUT -p tcp -d {ip_address} --dport 443 -j REDIRECT --to-port {host.http_port}
                run(wrap_command_with_sudo(["iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "-d", ip_address, "--dport", "443", "-j", "REDIRECT", "--to-port", str(host.https_port)]))
                run(wrap_command_with_sudo(["iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "-d", ip_address, "--dport", "443", "-j", "REDIRECT", "--to-port", str(host.https_port)]))

        # USE iproute2
        # https://askubuntu.com/questions/444124/how-to-add-a-loopback-interface/444128?_gl=1*t9x4hc*_ga*MTg3MDM0NTY2My4xNjYzODMzMzY3*_ga_S812YQPLT2*MTY2NTcxMjI4Ni4xMC4xLjE2NjU3MTMxMDYuMC4wLjA.#444128
        # https://unix.stackexchange.com/questions/353652/setting-up-a-development-environment-with-iptables
        # "dev" -> device, ie. adds to the loopback interface
        # ip addr add 127.0.0.2 dev lo
        #iptables -t nat -A PREROUTING -p tcp -d 127.0.0.2 --dport 80 -j REDIRECT --to-port 3000
        #iptables -t nat -A OUTPUT -p tcp -d 127.0.0.2 --dport 80 -j REDIRECT --to-port 3000

    def configure_mac(self, host_to_ip: dict[SyntheticHostDefinition, str]):
        for ip_address in host_to_ip.values():
            run(wrap_command_with_sudo(["ifconfig", "lo0", "alias", ip_address, "up"]))

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

                run(wrap_command_with_sudo(["pfctl", "-e", "-f", new_root_file.name]))

        return host_to_ip
