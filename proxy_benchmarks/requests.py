from abc import ABC, abstractmethod
from playwright.sync_api import sync_playwright
from requests import get

class BaseRequest(ABC):
    @abstractmethod
    def handle_request(self, url: str, proxy: str | None):
        pass

class PythonRequest(BaseRequest):
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

class ChromeRequestHeadfull(BaseRequest):
    def handle_request(self, url: str, proxy: str | None):
        with sync_playwright() as p:
            browser = p.chromium.launch(
                headless=False,
            )
            context = browser.new_context(
                **{
                    "proxy": {
                        "server": proxy,
                    }
                } if proxy else None,
                ignore_https_errors=True,
            )
            page = context.new_page()
            response = page.goto(url)
            assert response.ok
            browser.close()

class ChromeRequestHeadless(BaseRequest):
    def handle_request(self, url: str, proxy: str | None):
        pass
