from abc import ABC, abstractmethod
from contextlib import contextmanager
from pathlib import Path
from dataclasses import dataclass
from proxy_benchmarks.networking import is_socket_bound
from time import sleep


@dataclass
class CertificateAuthority:
    public: Path
    key: Path


class ProxyBase(ABC):
    def __init__(self, port):
        self.port = port

    @abstractmethod
    @contextmanager
    def launch(self):
        pass

    def wait_for_launch(self, timeout=20):
        # Wait for the socket to open
        while not is_socket_bound(self.port) and timeout > 0:
            print("Waiting for proxy port to open...")
            sleep(1)
            timeout -= 1
        if timeout == 0:
            raise TimeoutError("Timed out waiting for proxy to open")

    def wait_for_close(self, timeout=20):
        # Wait for the socket to open
        while is_socket_bound(self.port) and timeout > 0:
            print("Waiting for proxy port to close...")
            sleep(1)
            timeout -= 1
        if timeout == 0:
            raise TimeoutError("Timed out waiting for proxy to close")

    @property
    @abstractmethod
    def certificate_authority(self) -> CertificateAuthority:
        """
        Root CA that's used to generate the different client hosts.
        """
        pass

    @property
    @abstractmethod
    def short_name(self) -> str:
        pass

    @abstractmethod
    def __repr__(self) -> str:
        pass
