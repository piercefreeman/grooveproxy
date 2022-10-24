import pytest
from playwright.sync_api import sync_playwright

from groove.proxy import Groove


# We want this to recreate by default on every unit test to clear the state
@pytest.fixture(scope="function")
def proxy():
    proxy = Groove()
    with proxy.launch():
        # Before we yield to new client calls clear out any remaining cache items
        proxy.cache_clear()
        yield proxy

@pytest.fixture(scope="function")
def browser():
    with sync_playwright() as p:
        yield p.chromium.launch(
            headless=True,
        )
