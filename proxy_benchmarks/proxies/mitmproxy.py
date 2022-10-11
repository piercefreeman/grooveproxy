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
from proxy_benchmarks.proxies.base import ProxyBase, CertificateAuthority
from proxy_benchmarks.process import terminate_all
from proxy_benchmarks.assets import get_asset_path


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
        certificate_directory = get_asset_path("proxies/mitmproxy")

        process = Popen(
            f"poetry run mitmdump -s '{current_extension_path}' --listen-port {self.port} --set confdir={certificate_directory} --ssl-insecure",
            shell=True,
        )

        self.wait_for_launch()
        sleep(1)

        try:
            yield process
        finally:
            terminate_all(process)

            # Remove certificates from launch so we can explicitly test new credential generation

            self.wait_for_close()

    @property
    def certificate_authority(self) -> CertificateAuthority:
        return CertificateAuthority(
            public=get_asset_path("proxies/mitmproxy/mitmproxy-ca.crt"),
            key=get_asset_path("proxies/mitmproxy/mitmproxy-ca.key"),
        )

    @property
    def short_name(self) -> str:
        return "mitmproxy"

    def __repr__(self) -> str:
        return f"MitmProxy(port={self.port})"
