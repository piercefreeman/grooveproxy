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
    def __init__(self, headless, keep_open: bool = False):
        """
        :param headless: Whether to open the browser in headless mode.
        :param keep_open: Useful for debugging. Can optionally stop every time a
            page loads to better inspect the outgoing network requests and certificates.

        """
        self.headless = headless
        self.keep_open = keep_open

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
                # We explicitly don't set `ignore_https_errors=True` because we expect
                # that the setup pipeline will correctly configure our proxy certificates
                # and our test server certificates
            }

            context = browser.new_context(
                **payload
            )
            page = context.new_page()
            response = page.goto(url)

            if self.keep_open:
                # TODO: Update coloring in case it's only available in the scrollback history
                if input("Press any key to continue..."):
                    pass

            assert response.ok
            browser.close()

    def __repr__(self):
        return f"ChromeRequest(headless={self.headless})"
