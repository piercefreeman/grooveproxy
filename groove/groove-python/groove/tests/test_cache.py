from groove.proxy import Groove, CacheModeEnum
from playwright.sync_api import sync_playwright
from groove.tests.mock_server import MockPageDefinition, mock_server
from bs4 import BeautifulSoup
from uuid import uuid4


def test_cache_off():
    """
    Ensure the cache is off will route all requests
    """
    proxy = Groove()
    proxy.set_cache_mode(CacheModeEnum.OFF)

    # Leverage random identifiers for each test to ensure there isn't
    # data leakage from one unit test to the other
    request1_content = str(uuid4())
    request2_content = str(uuid4())

    with mock_server([
        MockPageDefinition(
            "/test",
            content=f"<html><body>{request1_content}</body></html>"
        ),
        MockPageDefinition(
            "/test",
            content=f"<html><body>{request2_content}</body></html>"
        ),
    ]) as mock_url:
        with sync_playwright() as p:
            browser = p.chromium.launch(
                headless=False,
            )
            context = browser.new_context(
                proxy={
                    "server": proxy.base_url_proxy,
                }
            )
            page = context.new_page()
            page.goto(f"{mock_url}/test")
            assert BeautifulSoup(page.content()).text.strip() == request1_content

            page.goto("https://freeman.vc")

            page.goto(f"{mock_url}/test")
            assert BeautifulSoup(page.content()).text.strip() == request2_content

def test_cache_aggressive():
    """
    Ensure the aggressive cache will cache all requests
    """
    proxy = Groove()

    # Clear previous cache records, if they exist
    proxy.set_cache_mode(CacheModeEnum.OFF)
    proxy.set_cache_mode(CacheModeEnum.AGGRESSIVE)

    request1_content = str(uuid4())
    request2_content = str(uuid4())

    with mock_server([
        MockPageDefinition(
            "/test",
            content=f"<html><body>{request1_content}</body></html>"
        ),
        MockPageDefinition(
            "/test",
            content=f"<html><body>{request2_content}</body></html>"
        ),
    ]) as mock_url:
        with sync_playwright() as p:
            browser = p.chromium.launch(
                headless=False,
            )
            context = browser.new_context(
                proxy={
                    "server": proxy.base_url_proxy,
                }
            )
            page = context.new_page()
            page.goto(f"{mock_url}/test")
            assert BeautifulSoup(page.content()).text.strip() == request1_content

            page.goto(f"{mock_url}/test")
            assert BeautifulSoup(page.content()).text.strip() == request1_content
