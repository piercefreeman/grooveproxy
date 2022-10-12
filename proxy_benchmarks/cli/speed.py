from json import dump
from pathlib import Path
from time import time

import pandas as pd
from click import (
    Path as ClickPath,
    group,
    option,
    pass_obj,
)
from requests import get
from tqdm import tqdm

from proxy_benchmarks.enums import MimicTypeEnum
from proxy_benchmarks.load_test import run_load_server
from proxy_benchmarks.networking import SyntheticHostDefinition, SyntheticHosts
from proxy_benchmarks.proxies.base import ProxyBase
from proxy_benchmarks.proxies.gomitmproxy import GoMitmProxy
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.proxies.martian import MartianProxy
from proxy_benchmarks.proxies.mitmproxy import MitmProxy
from proxy_benchmarks.proxies.node_http_proxy import NodeHttpProxy


@group()
def speed_test():
    pass


@speed_test.command()
@option("--samples", type=int, default=100)
@option("--data-path", type=ClickPath(dir_okay=True, file_okay=False), required=True)
@pass_obj
def execute(obj, samples, data_path):
    """
    Benchmark speed of certificate generation.

    """
    console = obj["console"]
    divider = obj["divider"]

    data_path = Path(data_path).expanduser()
    data_path.mkdir(exist_ok=True)

    proxies: list[ProxyBase] = [
        MitmProxy(),
        NodeHttpProxy(),
        GoMitmProxy(MimicTypeEnum.STANDARD),
        MartianProxy(),
        GoProxy(MimicTypeEnum.STANDARD),
    ]

    proxy_samples = []

    with run_load_server() as load_server_definition:
        synthetic_ip_addresses = SyntheticHosts(
            [
                SyntheticHostDefinition(
                    name="load-server",
                    http_port=load_server_definition["http"],
                    https_port=load_server_definition["https"],
                )
            ]
        ).configure()
        synthetic_ip_address = next(iter(synthetic_ip_addresses.values()))

        # Clear out any previously generated certificates by opening and then closing
        # the context manager
        console.print(f"{divider}\nCleaning up cached proxy certificates...\n{divider}", style="bold blue")
        for proxy in proxies:
            with proxy.launch():
                pass

        for proxy in proxies:
            console.print(f"{divider}\nWill perform certificate generation test with {proxy}...\n{divider}", style="bold blue")

            proxy_definition = {
                "http": f"http://localhost:{proxy.port}",
                "https": f"http://localhost:{proxy.port}",
            }

            for _ in tqdm(range(samples)):
                with proxy.launch():
                    start_time = time()
                    cold_start_response = get(
                        f"https://{synthetic_ip_address}/handle",
                        proxies=proxy_definition,
                        verify=proxy.certificate_authority.public,
                    )
                    cold_start_time = time() - start_time

                    start_time = time()
                    warm_start_response = get(
                        f"https://{synthetic_ip_address}/handle",
                        proxies=proxy_definition,
                        verify=proxy.certificate_authority.public,
                    )
                    warm_start_time = time() - start_time

                    proxy_samples.append(
                        dict(
                            proxy=proxy.short_name,
                            cold_start=cold_start_time,
                            cold_start_status=cold_start_response.status_code,
                            warm_start=warm_start_time,
                            warm_start_status=warm_start_response.status_code,                            
                        )
                    )

    with open(data_path / "raw.json", "w") as file:
        dump(proxy_samples, file)


@speed_test.command()
@option("--data-path", type=ClickPath(dir_okay=True, file_okay=False), required=True)
def analyze_2(data_path):
    data_path = Path(data_path).expanduser()

    df = pd.read_json(data_path / "raw.json")
    df = df.assign(
        difference=df.cold_start-df.warm_start,
    )

    # Confirm basic success statistics
    print(df.groupby("proxy")["cold_start_status", "warm_start_status"].value_counts())

    distribution_df = df.groupby("proxy")[["cold_start", "warm_start", "difference"]].describe().reset_index()
    distribution_df.to_csv("results_certificate_speed.csv")
