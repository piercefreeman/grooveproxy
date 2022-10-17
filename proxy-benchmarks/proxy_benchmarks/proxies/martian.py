from contextlib import contextmanager
from subprocess import Popen
from time import sleep

from proxy_benchmarks.assets import get_asset_path
from proxy_benchmarks.process import terminate_all
from proxy_benchmarks.proxies.base import CertificateAuthority, ProxyBase


class MartianProxy(ProxyBase):
    def __init__(self):
        super().__init__(port=6014)

    @contextmanager
    def launch(self):
        current_extension_path = get_asset_path("proxies/martian")
        process = Popen(["go", "run", ".", "--port", str(self.port)], cwd=current_extension_path)

        self.wait_for_launch()
        # Requires a bit more time to load than our other proxies
        sleep(2)

        try:
            yield process
        finally:
            terminate_all(process)

            # Wait for the socket to close
            self.wait_for_close()

    @property
    def certificate_authority(self) -> CertificateAuthority:
        return CertificateAuthority(
            public=get_asset_path("proxies/martian/ssl/ca.crt"),
            key=get_asset_path("proxies/martian/ssl/ca.key"),
        )

    @property
    def short_name(self) -> str:
        return "martian"

    def __repr__(self) -> str:
        return f"MartianProxy(port={self.port})"
