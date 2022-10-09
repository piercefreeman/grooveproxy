from abc import ABC, abstractmethod

from playwright.sync_api import sync_playwright
from requests import get


class RequestBase(ABC):
    @abstractmethod
    def handle_request(self, url: str, proxy: str | None):
        pass


class PythonRequest(RequestBase):
    def handle_request(self, url: str, proxy: str | None):
        response = get(
            url,
            proxies={
                "http": proxy,
                "https": proxy,
            } if proxy else None,
            verify=False,
        )
        assert response.ok

    def __repr__(self):
        return "PythonRequest()"


class ChromeRequest(RequestBase):
    def __init__(self, headless):
        self.headless = headless

    def handle_request(self, url: str, proxy: str | None):
        with sync_playwright() as p:
            browser = p.chromium.launch(
                headless=self.headless,
            )
            payload = {
                **({
                    "proxy": {
                        "server": proxy,
                    }
                } if proxy else {}),
                "ignore_https_errors": True,
            }
            print("PAYLOAD", payload)

            context = browser.new_context(
                **payload
            )
            page = context.new_page()
            response = page.goto(url)
            assert response.ok
            browser.close()

    def __repr__(self):
        return f"ChromeRequest(headless={self.headless})"
