from contextlib import contextmanager
from proxy_benchmarks.assets import get_asset_path
from subprocess import Popen
from proxy_benchmarks.networking import is_socket_bound
from time import sleep

@contextmanager
def launch_proxy(port=8080):
    current_extension_path = get_asset_path("node_http_proxy")
    process = Popen(f"npm run main --port {port}", shell=True, cwd=current_extension_path)

    # Wait for the proxy to spin up
    while not is_socket_bound("localhost", port):
        print("Waiting for proxy port launch...")
        sleep(1)

    sleep(1)

    try:
        yield process
    finally:
        process.terminate()
