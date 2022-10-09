from contextlib import contextmanager
from signal import SIGTERM
from subprocess import Popen
from time import sleep

from psutil import Process as PsutilProcess

from proxy_benchmarks.assets import get_asset_path
from proxy_benchmarks.networking import is_socket_bound
from proxy_benchmarks.proxies.base import ProxyBase


class GoMitmProxy(ProxyBase):
    @contextmanager
    def launch(self):
        current_extension_path = get_asset_path("gomitmproxy")
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
            # Terminate all spawned subprocesses, including those belonging to the go proxy
            signal = SIGTERM
            process = PsutilProcess(process.pid)
            for child in process.children(recursive=True):
                child.send_signal(signal)
            process.send_signal(signal)

    def __repr__(self) -> str:
        return f"GoMitmProxy(port={self.port})"
