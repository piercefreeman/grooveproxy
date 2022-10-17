import pytest
from rich.console import Console


@pytest.fixture(scope="session")
def cli_object():
    console = Console()

    return dict(
        console=console,
        divider="-"*console.width,
    )
