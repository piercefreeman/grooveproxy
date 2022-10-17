from collections import Counter, defaultdict
from contextlib import contextmanager
from dataclasses import dataclass
from functools import partial
from itertools import groupby
from multiprocessing import Process, Semaphore
from time import sleep
from typing import Any, Optional

import uvicorn
from fastapi import FastAPI
from fastapi.responses import Response


@dataclass
class MockPageDefinition:
    url: str
    status_code: int = 200
    content_type: str = "text/html"

    method: str = "get"
    headers: dict[str, str] | None = None

    # Optional function callable - one should be specified
    content: Optional[str] = None
    handler: Optional[Any] = None

def launch_server(payloads: list[MockPageDefinition], port: int, setup_callback: Optional[Any], allow_repeat_access: bool, semaphore: Semaphore):
    server = FastAPI()
    requests_by_url = defaultdict(int)

    if setup_callback is not None:
        setup_callback(server)
    
    def render_favicon():
        return "fav"

    def render_default(page_definitions: list[MockPageDefinition]):
        url = page_definitions[0].url
        existing_requests = requests_by_url[url]

        if len(page_definitions) > 1 and existing_requests >= len(page_definitions):
            raise ValueError(f"Accessing beyond bound of sequential definitions: `{url}`.")
        if len(page_definitions) == 1 and existing_requests > 0 and not allow_repeat_access:
            raise ValueError(f"You have already accessed this url: `{url}`.")

        if len(page_definitions) > 1:
            definition = page_definitions[existing_requests]
        else:
            definition = page_definitions[0]

        requests_by_url[url] += 1

        return Response(
            content=definition.content, 
            status_code=definition.status_code,
            media_type=definition.content_type,
            headers=definition.headers,
        )

    # Group by URL (maintaining the original order)
    definitions_by_url = {
        (url, method): [definition for _, definition in definitions]
        for (url, method), definitions in groupby(
            sorted(enumerate(payloads), key=lambda x: (x[1].url, x[1].method, x[0])),
            key=lambda x: (x[1].url, x[1].method)
        )
    }

    for (url, method), definitions in definitions_by_url.items():
        mount_fn = getattr(server, method)

        # Only one URL should be provided if a handler is active
        has_handler = any([definition.handler for definition in definitions])
        if has_handler and len(definitions) > 1:
            raise ValueError(f"Only one definition is supported when handler is active: `{url}`")

        mount_fn(url)(definitions[0].handler or partial(render_default, page_definitions=definitions))

    def not_found_fallback(path):
        raise ValueError(f"No page matching query: {path}")

    server.get("/favicon.ico")(render_favicon)
    # TODO: Switch to a 404 handler
    server.get("{path:path}")(not_found_fallback)
    semaphore.release()
    uvicorn.run(server, host="0.0.0.0", port=port)

@contextmanager
def mock_server(
    payloads: list[MockPageDefinition],
    port=6012,
    setup_callback=None,
    allow_repeat_access: bool = True,
):
    """
    Sets up a test server parameterized by MockPageDefinition in the background.

    :param setup_callback: If specified, will be passed the FastAPI server object once createed. This function
        will be called in the separate process.
    :parma allow_repeat_access: Allow the same URL to be accessed multiple times within the
        same mock session. If false will throw an error if already accessed. Only applies to entries
        that have one definition per URL. If multiple definitions are given per URL, will assume
        that you intend to iterate them.

    """
    process_ready_semaphore = Semaphore(0)

    process = Process(target=launch_server, args=(payloads, port, setup_callback, allow_repeat_access, process_ready_semaphore))
    process.start()

    with process_ready_semaphore:
        try:
            print("Launched mock server...")
            # Wait for process to actually spawn
            sleep(0.1)
            yield f"http://localhost:{port}"
        except Exception as e:
            raise e
        finally:
            process.kill()
