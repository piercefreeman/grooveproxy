from contextlib import contextmanager
from subprocess import Popen
from time import sleep

from proxy_benchmarks.assets import get_asset_path
from proxy_benchmarks.process import terminate_all
from proxy_benchmarks.proxies.base import CertificateAuthority, ProxyBase


class NodeHttpProxy(ProxyBase):
    def __init__(self):
        super().__init__(port=6014)

    @contextmanager
    def launch(self):
        current_extension_path = get_asset_path("proxies/node_http_proxy")
        # We need to launch with node and not npm, otherwise it won't receive the shutdown signal
        # and shutdown will time out
        process = Popen(f"node index.js --port {self.port}", shell=True, cwd=current_extension_path)

        self.wait_for_launch()
        sleep(1)

        try:
            yield process
        finally:
            terminate_all(process)

            # Delete the content on disk
            certificates_path = get_asset_path("proxies/node_http_proxy/.http-mitm-proxy/certs")
            keys_path = get_asset_path("proxies/node_http_proxy/.http-mitm-proxy/keys")
            filename_whitelist = {"ca.private.key", "ca.public.key", "ca.pem"}

            for root_path in [certificates_path, keys_path]:
                for path in root_path.iterdir():
                    if path.name not in filename_whitelist:
                        print(f"Will remove: {path}")
                        path.unlink()

            self.wait_for_close()

    @property
    def certificate_authority(self) -> CertificateAuthority:
        return CertificateAuthority(
            public=get_asset_path("proxies/node_http_proxy/.http-mitm-proxy/certs/ca.pem"),
            key=get_asset_path("proxies/node_http_proxy/.http-mitm-proxy/keys/ca.private.key"),
        )

    @property
    def short_name(self) -> str:
        return "node_http_proxy"

    def __repr__(self) -> str:
        return f"NodeHttpProxy(port={self.port})"
