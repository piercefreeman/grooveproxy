from contextlib import closing, contextmanager
from pathlib import Path
from socket import AF_INET, SOCK_STREAM, socket
from subprocess import PIPE, Popen, run

from psutil import net_if_addrs


def is_socket_bound(host, port) -> bool:
    with closing(socket(AF_INET, SOCK_STREAM)) as sock:
        if sock.connect_ex((host, port)) == 0:
            return False
        else:
            return True


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
    print(outputs.decode())
    print(errors.decode())
