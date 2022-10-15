from groove.proxy import Groove
from playwright.sync_api import sync_playwright

def test_tape():
    """
    Ensure the basic tape functions work correctly
    """
    proxy = Groove()
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

        # TODO: Parse html to avoid chrome formatting differences
        assert page.content().strip() == "<html><head></head><body>Mocked content</body></html>"

def test_multiple_requests():
    """
    Ensure mocked requests resolve in the same order
    """
    pass
