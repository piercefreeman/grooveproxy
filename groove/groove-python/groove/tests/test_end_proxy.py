from bs4 import BeautifulSoup
from playwright.sync_api import sync_playwright
from base64 import b64encode
import pytest

from groove.proxy import (
    Groove,
    TapeRecord,
    TapeRequest,
    TapeResponse,
    TapeSession,
)

AUTH_USERNAME = "test-username"
AUTH_PASSWORD = "test-password"


@pytest.mark.parametrize(
    "end_proxy,middle_proxy",
    [
        (
            # Unauthenticated end proxy
            Groove(port=6040, control_port=6041),
            Groove(port=6010, control_port=6011),
        ),
        #(
        #    # Authenticated end proxy
        #    # Currently failing because of Chromium not sending Auth headers on every request
        #    Groove(port=6040, control_port=6041, auth_username=AUTH_USERNAME, auth_password=AUTH_PASSWORD),
        #    Groove(port=6010, control_port=6011, proxy_username=AUTH_USERNAME, proxy_password=AUTH_PASSWORD),
        #)
    ]
)
def test_end_proxy(end_proxy, middle_proxy):
    """
    Ensure the proxy can forward to an end proxy
    """
    middle_proxy.proxy_server = end_proxy.base_url_proxy

    record = TapeRecord(
        request=TapeRequest(
            url="https://freeman.vc:443/",
            method="GET",
            headers={},
            body=b"",
        ),
        response=TapeResponse(
            status=200,
            headers={},
            body=b64encode(b"Test content")
        ),
    )

    with middle_proxy.launch():
        with end_proxy.launch():
            end_proxy.tape_load(
                TapeSession(
                    records=[
                        # Double requests for the two proxy requests
                        record,
                        record,
                    ]
                )
            )

            with sync_playwright() as p:
                browser = p.chromium.launch(
                    headless=False,
                )

                proxy_payload = {
                    "server": end_proxy.base_url_proxy,
                    **(
                        {
                            "username": end_proxy.auth_username,
                            "password": end_proxy.auth_password,
                        }
                        if end_proxy.auth_username and end_proxy.auth_password
                        else {}
                    ),
                }
                print("End proxy request payload", proxy_payload)

                # Make sure the end proxy has configured correctly
                context = browser.new_context(
                    proxy=proxy_payload,
                )
                page = context.new_page()
                page.goto("https://freeman.vc", timeout=5000)
                assert BeautifulSoup(page.content()).text.strip() == "Test content"

                # Make sure the middle proxy routes through the end proxy correctly
                context = browser.new_context(
                    proxy={
                        "server": middle_proxy.base_url_proxy,
                    }
                )
                page = context.new_page()
                page.goto("https://freeman.vc", timeout=5000)
                assert BeautifulSoup(page.content()).text.strip() == "Test content"