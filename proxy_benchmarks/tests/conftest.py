from rich.console import Console
import pytest

@pytest.fixture(scope="session")
def cli_object():
    return dict(
        console=Console(),
        divider="---",
    )
