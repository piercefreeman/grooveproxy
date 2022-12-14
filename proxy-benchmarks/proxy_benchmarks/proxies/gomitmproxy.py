from contextlib import contextmanager
from subprocess import Popen
from time import sleep

from proxy_benchmarks.assets import get_asset_path
from proxy_benchmarks.enums import MimicTypeEnum
from proxy_benchmarks.process import terminate_all
from proxy_benchmarks.proxies.base import CertificateAuthority, ProxyBase


proxy_configurations = {
    MimicTypeEnum.STANDARD: dict(
        project_path="gomitmproxy",
        port=6010,
    ),
    MimicTypeEnum.MIMIC: dict(
        project_path="gomitmproxy-mimic",
        port=6011,
    )
}

class GoMitmProxy(ProxyBase):
    def __init__(self, proxy_type: MimicTypeEnum):
        configuration = proxy_configurations[proxy_type]

        super().__init__(port=configuration["port"])
        self.project_path = configuration["project_path"]

    @contextmanager
    def launch(self):
        current_extension_path = get_asset_path(f"proxies/{self.project_path}")
        process = Popen(["go", "run", ".", "--port", str(self.port)], cwd=current_extension_path)

        self.wait_for_launch()
        # Requires a bit more time to load than our other proxies
        sleep(2)

        try:
            yield process
        finally:
            terminate_all(process)

            # Wait for the socket to close
            self.wait_for_close(60)

    @property
    def certificate_authority(self) -> CertificateAuthority:
        return CertificateAuthority(
            public=get_asset_path(f"proxies/{self.project_path}/ssl/ca.crt"),
            key=get_asset_path(f"proxies/{self.project_path}/ssl/ca.key"),
        )

    @property
    def short_name(self) -> str:
        return self.project_path

    def __repr__(self) -> str:
        return f"GoMitmProxy(port={self.port},version={self.project_path})"
