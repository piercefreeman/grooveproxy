from contextlib import contextmanager
from subprocess import Popen
from time import sleep

from proxy_benchmarks.assets import get_asset_path
from proxy_benchmarks.networking import is_socket_bound
from proxy_benchmarks.proxies.base import ProxyBase


class NodeHttpProxy(ProxyBase):
    @contextmanager
    def launch(self):
        current_extension_path = get_asset_path("proxies/node_http_proxy")
        process = Popen(f"npm run main --port {self.port}", shell=True, cwd=current_extension_path)

        # Wait for the proxy to spin up
        while not is_socket_bound("localhost", self.port):
            print("Waiting for proxy port launch...")
            sleep(1)

        sleep(1)

        try:
            yield process
        finally:
            process.terminate()

    @property
    def short_name(self) -> str:
        return "node_http_proxy"

    def __repr__(self) -> str:
        return f"NodeHttpProxy(port={self.port})"
