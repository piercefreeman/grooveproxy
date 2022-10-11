from contextlib import contextmanager
from subprocess import Popen
from time import sleep

from proxy_benchmarks.assets import get_asset_path
from proxy_benchmarks.networking import is_socket_bound
from proxy_benchmarks.process import terminate_all
from proxy_benchmarks.proxies.base import ProxyBase


class GoProxy(ProxyBase):
    @contextmanager
    def launch(self):
        current_extension_path = get_asset_path("proxies/goproxy")
        process = Popen(f"go run . --port {self.port}", shell=True, cwd=current_extension_path)

        # Wait for the proxy to spin up
        while not is_socket_bound("localhost", self.port):
            print("Waiting for proxy port launch...")
            sleep(1)

        # Requires a bit more time to load than our other proxies
        sleep(2)

        try:
            yield process
        finally:
            terminate_all(process)

    @property
    def short_name(self) -> str:
        return "goproxy"

    def __repr__(self) -> str:
        return f"GoProxy(port={self.port})"
