"""
Basic skeleton of a mitmproxy addon.

Run as follows: 

"""
from pathlib import Path
from subprocess import Popen
from time import sleep

from mitmproxy import ctx

from proxy_benchmarks.networking import is_socket_bound


class Counter:
    def __init__(self):
        self.num = 0

    def request(self, flow):
        print("FLOW", flow)
        self.num = self.num + 1
        ctx.log.info("We've seen %d flows" % self.num)


addons = [Counter()]



def launch_proxy(port=8080):
    current_extension_path = Path(__file__).resolve()
    process = Popen(f"poetry run mitmdump -s '{current_extension_path}' --listen-port {port}", shell=True)

    # Wait for the proxy to spin up
    while not is_socket_bound("localhost", port):
        print("Waiting for proxy port launch...")
        sleep(1)

    return process
