from bs4 import BeautifulSoup
from playwright.sync_api import sync_playwright

from groove.proxy import TapeSession


def test_tape(proxy):
    """
    Ensure the basic tape functions work correctly
    """
    proxy.tape_start()

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
        page.goto("https://freeman.vc")

    modified_records = 0
    session = proxy.tape_get()
    assert len(session.records) > 0

    for record in session.records:
        if record.request.url == "https://freeman.vc:443/":
            record.response.body = "Mocked content".encode()
            modified_records += 1
    assert modified_records == 1

    proxy.tape_load(session)

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
        page.goto("https://freeman.vc")

        assert BeautifulSoup(page.content()).text.strip() == "Mocked content"

def test_multiple_requests():
    """
    Ensure mocked requests resolve in the same order
    """
    pass
