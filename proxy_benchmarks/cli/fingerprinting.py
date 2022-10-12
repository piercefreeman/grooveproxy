from contextlib import contextmanager
from dataclasses import asdict
from functools import partial
from json import loads
from pathlib import Path
from socket import gethostbyname
from subprocess import PIPE, run
from tempfile import TemporaryDirectory
from time import sleep
from typing import Callable
from urllib.parse import urlparse

from click import (
    Path as ClickPath,
    group,
    option,
    pass_obj,
)
from rich.table import Table

from proxy_benchmarks.fingerprinting import CaptureParser, Ja3Record, ja3_by_ip
from proxy_benchmarks.networking import capture_network_traffic
from proxy_benchmarks.proxies.base import ProxyBase
from proxy_benchmarks.proxies.gomitmproxy import GoMitmProxy
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.proxies.martian import MartianProxy
from proxy_benchmarks.proxies.mitmproxy import MitmProxy
from proxy_benchmarks.proxies.node_http_proxy import NodeHttpProxy
from proxy_benchmarks.requests import ChromeRequest, PythonRequest, RequestBase


# To test TCP connection, we need a valid https url
TEST_TCP_URL = "https://freeman.vc"


@group()
def fingerprint():
    pass


@fingerprint.command()
@option("--output-directory", type=ClickPath(), required=False)
@pass_obj
def execute(obj, output_directory):
    """
    :param output_directory: Optional parameter. If specified, will create named packet logs
        in the given directory. Otherwise (by default) will create in a temporary path and
        garbage collect it once tests are completed.

    """
    output_directory = Path(output_directory).expanduser()
    output_directory.mkdir(exist_ok=True)

    console = obj["console"]
    divider = obj["divider"]

    # Ensure we have sudo permissions
    print("proxy-benchmarks needs to capture network traffic...")
    run(f"sudo echo 'Confirmation success...\n'", shell=True)

    proxies: list[ProxyBase] = [
        MitmProxy(),
        NodeHttpProxy(),
        GoMitmProxy(),
        MartianProxy(),
        GoProxy(),
    ]

    runners: list[RequestBase] = [
        PythonRequest(),
        ChromeRequest(headless=True),
        ChromeRequest(headless=False),
    ]

    for proxy in proxies:
        print(f"{proxy}: Launched proxy...")

        for runner in runners:
            fingerprint_by_proxy = {}

            # Some proxies have caching built-in, so we should disable this during testing to make
            # sure we start with a clean slate for later tests
            with proxy.launch():
                # Compare fingerprint signatures with and without proxy
                for proxy_url in [f"http://localhost:{proxy.port}", None]:
                    divider = "-" * console.width
                    console.print(f"{divider}\nTesting {runner} with proxy {proxy} ({proxy_url})\n{divider}", style="bold blue")

                    with optional_output_path(output_directory, proxy, runner, proxy_url) as capture_path:
                        fingerprints = get_fingerprint(
                            TEST_TCP_URL,
                            request_fn=partial(
                                runner.handle_request,
                                proxy=proxy_url,
                            ),
                            capture_path=capture_path,
                        )

                    # If multiple fingerprints came back for the domain (because of multiple outbound requests on different
                    # TCP connections), ensure these are all the same. We expect the same request type
                    # will have the same fingerprint over time so this is mostly a sanity check.
                    fingerprint_digests = {record.digest for record in fingerprints}
                    if len(fingerprint_digests) > 1:
                        console.print(f"Fingerprint values are not consistent across requests: `{fingerprint_digests}`.", style="bold red")
                        pass

                    console.print(f"Results {runner} {proxy} ({proxy_url}): {fingerprint_digests}")
                    fingerprint_by_proxy[proxy_url] = fingerprint_digests

                # Ensure the fingerprints are the same for the same request type, regardless of proxy
                fingerprint_values = {frozenset(digests) for digests in fingerprint_by_proxy.values()}
                if len(fingerprint_values) > 1:
                    console.print(f"\nFingerprint values are not consistent across proxies:", style="bold red")
                    for proxy_url, digests in fingerprint_by_proxy.items():
                        console.print(f"  {proxy} {proxy_url}: {digests}")


@fingerprint.command()
@option("--file1", type=ClickPath(exists=True), required=True)
@option("--file2", type=ClickPath(exists=True), required=True)
@pass_obj
def compare(obj, file1, file2):
    """
    Requires you to have the `tshark` utility provided by Wireshark to format the stat files.

    """
    console = obj["console"]
    divider = obj["divider"]
    parser = CaptureParser(console)

    test_host = urlparse(TEST_TCP_URL)
    search_ip = gethostbyname(test_host.netloc)
    console.print(f"Searching for IP: {search_ip}")

    file1 = Path(file1).expanduser()
    file2 = Path(file2).expanduser()

    # By default some json list outputs will share a key
    # This was particularly problematic for `tls.handshake.ciphersuite` values. The schema defines
    # it as a dictionary but the keys are the same for every entry, so upon json parsing it
    # will be collapsed into one record. This was reported [here](https://www.wireshark.org/lists/wireshark-dev/201706/msg00058.html)
    # and fixed in a subsequent patch that's only enabled when passing the `--no-duplicate-keys` key.

    file1_output = run(["tshark", "-r", file1, "-T", "json", "--no-duplicate-keys"], stdout=PIPE, stderr=PIPE)
    file2_output = run(["tshark", "-r", file2, "-T", "json", "--no-duplicate-keys"], stdout=PIPE, stderr=PIPE)

    file1_capture = loads(file1_output.stdout)
    file2_capture = loads(file2_output.stdout)

    file1_hello = parser.get_hello_client(file1_capture, search_ip)
    file2_hello = parser.get_hello_client(file2_capture, search_ip)

    if not file1_hello:
        console.print(f"Could not find TCP hello world in {file1}", style="bold red")
        return
    if not file2_hello:
        console.print(f"Could not find TCP hello world in {file2}", style="bold red")
        return

    file1_ja3 = parser.build_ja3_payload(file1_hello)
    file2_ja3 = parser.build_ja3_payload(file2_hello)

    # Name the table columns after the file names
    col1_name = file1.name
    col2_name = file2.name

    for key in asdict(file1_ja3).keys():
        console.print(f"{divider}\nCompare `{key}`\n{divider}", style="bold blue")

        table = Table(show_header=True, header_style="bold magenta")
        table.add_column("Value")
        table.add_column(col1_name)
        table.add_column(col2_name)

        file1_value = getattr(file1_ja3, key)
        file2_value = getattr(file2_ja3, key)

        if isinstance(file1_value, set):
            for value in file1_value | file2_value:
                style = "bold green" if value in file1_value and value in file2_value else "bold red"
                table.add_row(
                    str(value),
                    f"[{style}]{value in file1_value}[/{style}]",
                    f"[{style}]{value in file2_value}[/{style}]",
                )
        else:
            # Compare standard value
            values_equal = file1_value == file2_value
            style = "bold green" if values_equal else "bold red"
            table.add_row(
                "",
                f"[{style}]{file1_value}[/{style}]",
                f"[{style}]{file2_value}[/{style}]",
            )

        console.print(table)


@contextmanager
def optional_output_path(output_directory: Path, proxy: ProxyBase, runner: RequestBase, proxy_url: str | None) -> Path:
    if output_directory:
        test_type = "proxy" if proxy_url else "no_proxy"
        yield output_directory / f"{proxy.short_name}-{runner.short_name}-{test_type}.pcap"
    else:
        with TemporaryDirectory() as directory:
            yield Path(directory) / "capture.pcap"


def get_fingerprint(url: str, request_fn: Callable[[str], None], capture_path: Path) -> dict[str, list[Ja3Record]]:
    """
    Given a URL to test, and a function which issues a network request, return a dictionary
    of IP addresses to a list of JA3 fingerprints.

    """
    with capture_network_traffic(capture_path):
        print("Capturing traffic...")
        request_fn(url)

        # We notice an occasional lag in writing capture details to disk, sleep to allow
        # the process to catch up
        sleep(2)

    network_records = ja3_by_ip(capture_path)

    # We only care about the ones to our test domain, since other network requests
    # might have been made by the system daemons or user in this capture interval
    test_host = urlparse(url)
    search_ip = gethostbyname(test_host.netloc) 

    return network_records[search_ip]
