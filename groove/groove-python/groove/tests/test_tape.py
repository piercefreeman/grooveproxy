from base64 import b64encode
from uuid import uuid4

from bs4 import BeautifulSoup

from groove.proxy import TapeRecord, TapeRequest, TapeResponse, TapeSession


def test_tape(proxy, browser):
    """
    Ensure the basic tape functions work correctly
    """
    proxy.tape_start()

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

    context = browser.new_context(
        proxy={
            "server": proxy.base_url_proxy,
        }
    )
    page = context.new_page()
    page.goto("https://freeman.vc")

    assert BeautifulSoup(page.content()).text.strip() == "Mocked content"


def test_multiple_requests(proxy, browser):
    """
    Ensure mocked requests resolve in the same order
    """
    response_1 = str(uuid4())
    response_2 = str(uuid4())

    records = [
        TapeRecord(
            request=TapeRequest(
                url="https://freeman.vc:443/",
                method="GET",
                headers={},
                body=b"",
            ),
            response=TapeResponse(
                status=200,
                headers={},
                body=b64encode(response.encode())
            ),
        )
        for response in [response_1, response_2]
    ]

    proxy.tape_load(
        TapeSession(
            records=records
        )
    )

    context = browser.new_context(
        proxy={
            "server": proxy.base_url_proxy,
        }
    )
    page = context.new_page()

    page.goto("https://freeman.vc")
    assert BeautifulSoup(page.content()).text.strip() == response_1

    page.goto("https://freeman.vc")
    assert BeautifulSoup(page.content()).text.strip() == response_2
