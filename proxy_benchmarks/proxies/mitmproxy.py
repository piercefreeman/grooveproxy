"""
Basic skeleton of a mitmproxy addon.

Run as follows: 

"""
from contextlib import contextmanager
from pathlib import Path
from subprocess import Popen
from time import sleep

from mitmproxy import ctx, http

from proxy_benchmarks.assets import get_asset_path
from proxy_benchmarks.process import terminate_all
from proxy_benchmarks.proxies.base import CertificateAuthority, ProxyBase


class Counter:
    def __init__(self):
        self.num = 0

    def request(self, flow: http.HTTPFlow):
        print("FLOW", flow)
        self.num = self.num + 1
        ctx.log.info("We've seen %d flows" % self.num)

        # if flow.request.pretty_url == "https://example.com/path":
        #     flow.response = http.Response.make(
        #         200,
        #         b"Hello World",
        #         {"Content-Type": "text/html"},
        #     )

    def response(self, flow: http.HTTPFlow):
        # flow.response.content += b"\nInjected content"
        pass


addons = [Counter()]


class MitmProxy(ProxyBase):
    def __init__(self):
        super().__init__(port=6013)

    @contextmanager
    def launch(self):
        current_extension_path = Path(__file__).resolve()
        certificate_directory = get_asset_path("proxies/mitmproxy")

        process = Popen(
            # NOTE: Even though our local testing server validates in the system keychain, mitmdump appears to
            # do a separate validation and throws a 502 bad gateway error when using locally signed certificates.
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
