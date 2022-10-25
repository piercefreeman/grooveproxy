from base64 import b64encode

import pytest
from bs4 import BeautifulSoup
from playwright._impl._api_types import Error as PlaywrightError
from requests import get

from groove.assets import get_asset_path
from groove.proxy import Groove
from groove.tape import TapeRecord, TapeRequest, TapeResponse, TapeSession

AUTH_USERNAME = "test-username"
AUTH_PASSWORD = "test-password"


@pytest.mark.xfail()
def test_auth_requests():
    proxy = Groove(port=6040, control_port=6041, auth_username=AUTH_USERNAME, auth_password=AUTH_PASSWORD)

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

    with proxy.launch():
        proxy.tape_load(
            TapeSession(
                records=[record]
            )
        )

        response = get(
            "https://freeman.vc",
            proxies={
                "http": f"http://{proxy.auth_username}:{proxy.auth_password}@localhost:{proxy.port}",
                "https": f"http://{proxy.auth_username}:{proxy.auth_password}@localhost:{proxy.port}",
            },
            verify=get_asset_path("ssl/ca.crt"),
        )
        assert response.ok
        assert BeautifulSoup(response.content, features="html.parser").strip() == "Test content"


@pytest.mark.xfail()
def test_auth_chromium(browser):
    """
    Ensure the proxy can forward to an end proxy
    """
    proxy = Groove(port=6040, control_port=6041, auth_username=AUTH_USERNAME, auth_password=AUTH_PASSWORD)

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

    with proxy.launch():
        proxy.tape_load(
            TapeSession(
                records=[record]
            )
        )

        # Make sure the end proxy has configured correctly
        context = browser.new_context(
            proxy={
                "server": proxy.base_url_proxy,
                "username": proxy.auth_username,
                "password": proxy.auth_password,
            },
        )
        page = context.new_page()
        page.goto("https://freeman.vc", timeout=5000)
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == "Test content"
