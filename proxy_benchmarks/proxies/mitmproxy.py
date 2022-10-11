"""
Basic skeleton of a mitmproxy addon.

Run as follows: 

"""
from contextlib import contextmanager
from pathlib import Path
from subprocess import Popen
from time import sleep

from mitmproxy import ctx

from proxy_benchmarks.networking import is_socket_bound
from proxy_benchmarks.proxies.base import ProxyBase


class Counter:
    def __init__(self):
        self.num = 0

    def request(self, flow):
        print("FLOW", flow)
        self.num = self.num + 1
        ctx.log.info("We've seen %d flows" % self.num)


addons = [Counter()]


class MitmProxy(ProxyBase):
    @contextmanager
    def launch(self):
        current_extension_path = Path(__file__).resolve()
        process = Popen(f"poetry run mitmdump -s '{current_extension_path}' --listen-port {self.port}", shell=True)

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
        return "mitmproxy"

    def __repr__(self) -> str:
        return f"MitmProxy(port={self.port})"
