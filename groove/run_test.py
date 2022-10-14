"""
poetry run python run_test.py

"""
from playwright.sync_api import sync_playwright
from time import sleep

with sync_playwright() as p:
    browser = p.chromium.launch(
        headless=False,
    )
    context = browser.new_context(
        proxy={
            "server": "http://localhost:6010"
        }
    )
    page = context.new_page()
    response = page.goto("https://freeman.vc")

    sleep(10000)
