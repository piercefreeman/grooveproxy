import pytest

from groove.proxy import Groove
from playwright.sync_api import sync_playwright


# We want this to recreate by default on every unit test to clear the state
@pytest.fixture(scope="function")
def proxy():
    proxy = Groove()
    with proxy.launch():
        yield proxy

@pytest.fixture(scope="function")
def browser():
    with sync_playwright() as p:
        yield p.chromium.launch(
            headless=True,
        )
