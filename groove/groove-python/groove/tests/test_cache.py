from uuid import uuid4

from bs4 import BeautifulSoup

from groove.proxy import CacheModeEnum
from groove.tests.mock_server import MockPageDefinition, mock_server


def test_cache_off(proxy, context):
    """
    Ensure the cache is off will route all requests
    """
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
        page = context.new_page()
        page.goto(f"{mock_url}/test")
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == request1_content

        page.goto(f"{mock_url}/test")
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == request2_content


def test_cache_standard(proxy, context):
    """
    Ensure the cache respects server headers
    """
    proxy.set_cache_mode(CacheModeEnum.STANDARD)

    request1_content = str(uuid4())
    request2_content = str(uuid4())

    with mock_server([
        MockPageDefinition(
            "/test",
            content=f"<html><body>{request1_content}</body></html>",
            headers={
                "Cache-Control": "max-age=604800",
            }
        ),
        MockPageDefinition(
            "/test",
            content=f"<html><body>{request2_content}</body></html>",
            headers={
                "Cache-Control": "max-age=604800",
            }
        ),
    ]) as mock_url:
        page = context.new_page()
        page.goto(f"{mock_url}/test")
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == request1_content

        # We should never hit the second definition because of the first requests' headers
        page.goto(f"{mock_url}/test")
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == request1_content


def test_cache_aggressive(proxy, context):
    """
    Ensure the aggressive cache will cache all requests
    """
    # Clear previous cache records, if they exist
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
        page = context.new_page()
        page.goto(f"{mock_url}/test")
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == request1_content

        page.goto(f"{mock_url}/test")
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == request1_content

def test_cache_aggressive_get(proxy, context):
    """
    Ensure the aggressive cache will cache GET request but not POST requests
    """
    # Clear previous cache records, if they exist
    proxy.set_cache_mode(CacheModeEnum.AGGRESSIVE_GET)

    request1_content = str(uuid4())
    request2_content = str(uuid4())
    request3_content = str(uuid4())

    with mock_server([
        MockPageDefinition(
            "/test",
            content=f"<html><body>{request1_content}<form method='POST'><input type='submit' id='submit-button' /></form></body></html>"
        ),
        MockPageDefinition(
            "/test",
            content=f"<html><body>{request2_content}</body></html>"
        ),
        MockPageDefinition(
            "/test",
            content=f"<html><body>{request3_content}</body></html>",
            method="post",
        ),
    ]) as mock_url:
        page = context.new_page()
        page.goto(f"{mock_url}/test")
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == request1_content

        # GET requests should cache
        page.goto(f"{mock_url}/test")
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == request1_content

        # But POST requests should not; issue via form submission
        page.click("#submit-button")
        assert BeautifulSoup(page.content(), features="html.parser").text.strip() == request3_content
