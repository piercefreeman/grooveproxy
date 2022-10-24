import pytest
from playwright._impl._api_types import Error as PlaywrightError

from groove.proxy import ProxyFailureError


@pytest.mark.xfail(reason="TLS certificate blocked", raises=ProxyFailureError)
def test_tls_addons(proxy, context):
    """
    Test TLS addons
    """
    page = context.new_page()

    try:
        page.goto("https://www.google.com:443/")
    except PlaywrightError as e:
        if "net::ERR_EMPTY_RESPONSE" in e.message:
            raise ProxyFailureError()
