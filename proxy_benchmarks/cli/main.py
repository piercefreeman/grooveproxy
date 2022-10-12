from click import group, pass_context
from rich.console import Console

from proxy_benchmarks.cli.fingerprinting import fingerprint
from proxy_benchmarks.cli.load import load_test
from proxy_benchmarks.cli.speed import speed_test
from proxy_benchmarks.cli.ssl_validity import basic_ssl_test


@group()
@pass_context
def main(ctx):
    console = Console(soft_wrap=True)

    ctx.obj = dict(
        console=console,
        divider="-" * console.width
    )

main.add_command(fingerprint)
main.add_command(load_test)
main.add_command(speed_test)
main.add_command(basic_ssl_test)
