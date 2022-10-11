from contextlib import contextmanager
from subprocess import Popen
from time import sleep

from proxy_benchmarks.assets import get_asset_path
from proxy_benchmarks.networking import is_socket_bound
from proxy_benchmarks.proxies.base import ProxyBase, CertificateAuthority


class NodeHttpProxy(ProxyBase):
    @contextmanager
    def launch(self):
        current_extension_path = get_asset_path("proxies/node_http_proxy")
        process = Popen(f"npm run main --port {self.port}", shell=True, cwd=current_extension_path)

        self.wait_for_launch()
        sleep(1)

        try:
            yield process
        finally:
            process.terminate()
            self.wait_for_close()

    @property
    def certificate_authority(self) -> CertificateAuthority:
        return CertificateAuthority(
            public=get_asset_path("proxies/node_http_proxy/.http-mitm-proxy/keys/ca.private.key"),
            key=get_asset_path("proxies/node_http_proxy/.http-mitm-proxy/keys/ca.public.key"),
        )

    @property
    def short_name(self) -> str:
        return "node_http_proxy"

    def __repr__(self) -> str:
        return f"NodeHttpProxy(port={self.port})"
