"""
poetry run python run_test.py

"""
from playwright.sync_api import sync_playwright
from time import sleep
from requests import post
from gzip import decompress, compress
from json import loads, dumps
from base64 import b64decode, b64encode

PROXY_URL = "http://localhost:6010"
CONTROL_URL = "http://localhost:5010"

with sync_playwright() as p:
    response = post(f"{CONTROL_URL}/api/tape/record")
    assert response.json()["success"] == True

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

    # Get the tape
    tape_response = post(f"{CONTROL_URL}/api/tape/retrieve")
    tape_data = tape_response.content

    tape_contents = loads(decompress(tape_data))
    main_page_record = [
        record
        for record in tape_contents
        if record["Request"]["Url"] == "https://freeman.vc:443/"
    ]
    assert len(main_page_record) == 1
    main_page_record = main_page_record[0]

    main_page_body = b64decode(main_page_record["Response"]["Body"])
    print(main_page_body)

    # Manipulate the main body content
    new_tape = [
        {
            "Request": main_page_record["Request"],
            "Response": {
                **main_page_record["Response"],
               "Body": b64encode("Mocked contents".encode()).decode(),
            }
        }
    ]
    new_tape_data = compress(dumps(new_tape).encode())

    # Start the playback
    tape_response = post(f"{CONTROL_URL}/api/tape/load", files={"file": new_tape_data})
    assert tape_response.json()["success"] == True

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

    # TODO: Parse html to avoid chrome formatting differences
    assert page.content().strip() == "<html><head></head><body>Mocked contents</body></html>"

    sleep(10000)
