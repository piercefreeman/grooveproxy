from abc import ABC, abstractmethod

from playwright.sync_api import sync_playwright
from requests import get
from proxy_benchmarks.io import is_docker


class RequestBase(ABC):
    @property
    @abstractmethod
    def short_name(self) -> str:
        pass

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

    @property
    def short_name(self) -> str:
        return "python"

    def __repr__(self) -> str:
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
            page_load_exception = None
            try:
                response = page.goto(url)
            except Exception as e:
                print("Exception encountered:", e)
                page_load_exception = e

            if self.keep_open:
                # TODO: Update coloring in case it's only available in the scrollback history
                if input("Press any key to continue..."):
                    pass

            # Wait until after halting the browser to throw
            if page_load_exception:
                raise page_load_exception

            assert response.ok
            browser.close()

    @property
    def short_name(self) -> str:
        return "chrome_headless" if self.headless else "chrome_headfull"

    def __repr__(self) -> str:
        return f"ChromeRequest(headless={self.headless})"
