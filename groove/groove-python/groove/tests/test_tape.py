from base64 import b64encode
from uuid import uuid4

from bs4 import BeautifulSoup
from functools import partial
from requests import get

from groove.tape import TapeRecord, TapeRequest, TapeResponse, TapeSession
from groove.tests.mock_server import MockPageDefinition, mock_server


def test_tape_global(proxy, browser):
    """
    Ensure the basic tape functions work correctly
    """
    proxy.tape_start()

    # Explicitly use different contexts because Chromium will cache this page client side
    context = browser.new_context(
        proxy={
            "server": proxy.base_url_proxy,
        }
    )
    page = context.new_page()
    page.goto("https://freeman.vc")
    page.close()

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

    assert BeautifulSoup(page.content(), features="html.parser").text.strip() == "Mocked content"


def test_tape_id(proxy, session):
    """
    Ensure tapes can be recorded separately
    """
    proxy.tape_start()

    with mock_server([
        MockPageDefinition(
            "/test1",
            content=f"<html><body>Request 1</body></html>"
        ),
        MockPageDefinition(
            "/test2",
            content=f"<html><body>Request 2</body></html>"
        ),
    ]) as mock_url:
        response1 = session.get(f"{mock_url}/test1", headers={"Tape-ID": "Tape1"})
        assert response1.ok
        response2 = session.get(f"{mock_url}/test2", headers={"Tape-ID": "Tape2"})
        assert response2.ok

    session1 = proxy.tape_get("Tape1")
    session2 = proxy.tape_get("Tape2")
    assert len(session1.records) == 1
    assert len(session2.records) == 1

    assert session1.records[0].request.url == f"{mock_url}/test1"
    assert session2.records[0].request.url == f"{mock_url}/test2"


def test_multiple_requests(proxy, context):
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

    page = context.new_page()

    page.goto("https://freeman.vc")
    assert BeautifulSoup(page.content(), features="html.parser").text.strip() == response_1

    page.goto("https://freeman.vc")
    assert BeautifulSoup(page.content(), features="html.parser").text.strip() == response_2
