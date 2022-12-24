import pytest
from playwright._impl._api_types import Error as PlaywrightError

from groove.proxy import ProxyFailureError, Groove


def test_tls_addons(proxy: Groove, context):
    """
    Test that our TLS payload has a field listing the installed signature extensions. We know
    that Google specifically checks for ALPS (ApplicationSettingsExtension) so we try to render
    the homepage here.

    """
    page = context.new_page()

    proxy.tape_start()

    try:
        page.goto("https://www.google.com:443/")
    except PlaywrightError as e:
        if "net::ERR_EMPTY_RESPONSE" in e.message:
            raise ProxyFailureError()

    # Get the page
    tape_session = proxy.tape_get()

    assert len(tape_session.records) > 1

    main_page = [
        record
        for record in tape_session.records
        if record.request.url.strip("/") == "https://www.google.com:443"
    ]

    assert len(main_page[0].response.body) > 1000
