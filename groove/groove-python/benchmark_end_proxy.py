"""
Benchmark the load performance of a 3rd party proxy provider

"""
from time import time

from click import command, option, secho
from groove.enums import CacheModeEnum
from playwright.sync_api import sync_playwright

from groove.dialer import (DefaultLocalPassthroughDialer, DialerDefinition,
                           ProxyDefinition)
from groove.proxy import Groove


def handle(route, request):
    resource_type = request.resource_type

    # override headers
    headers = {
        **request.headers,
        "Resource-Type": resource_type,
    }
    route.continue_(headers=headers)


@command()
@option("--url", required=True)
@option("--proxy-server", required=True)
@option("--proxy-username", required=True)
@option("--proxy-password", required=True)
def benchmark(url, proxy_server, proxy_username, proxy_password):
    groove = Groove(port=6040, control_port=6041)

    with groove.launch():
        with sync_playwright() as p:
            groove.set_cache_mode(CacheModeEnum.OFF)
            groove.dialer_load(
                [
                    DefaultLocalPassthroughDialer(),
                    DialerDefinition(
                        priority=DefaultLocalPassthroughDialer().priority - 1,
                        proxy=ProxyDefinition(
                            url=proxy_server,
                            username=proxy_username,
                            password=proxy_password,
                        ),
                    )
                ]
            )

            browser = p.chromium.launch(
                headless=False,
            )

            context = browser.new_context(
                proxy={
                    "server": groove.base_url_proxy,
                }
            )

            page = context.new_page()

            page.route("**/*", handle)

            start = time()
            page.goto(url, timeout=60000)
            end = time()
            print(f"Time taken: {end - start}")

            browser.close()

if __name__ == '__main__':
    benchmark()
