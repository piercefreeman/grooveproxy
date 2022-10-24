from contextlib import contextmanager
from enum import Enum
from subprocess import Popen
from sysconfig import get_config_var
from time import sleep
from urllib.parse import urljoin

from requests import Session

from groove.assets import get_asset_path
from groove.dialer import DefaultInternetDialer, DialerDefinition
from groove.enums import CacheModeEnum
from groove.tape import TapeSession


class ProxyFailureError(Exception):
    pass


class Groove:
    def __init__(
        self,
        command_timeout: int = 5,
        port: int = 6010,
        control_port: int = 6011,
        auth_username: str | None = None,
        auth_password: str | None = None,
    ):
        self.session = Session()

        self.port = port
        self.control_port = control_port
        self.auth_username = auth_username
        self.auth_password = auth_password

        self.base_url_proxy = f"http://localhost:{port}"
        self.base_url_control = f"http://localhost:{control_port}"

        self.timeout = command_timeout

    @contextmanager
    def launch(self):
        parameters = {
            "--port": self.port,
            "--control-port": self.control_port,
            "--auth-username": self.auth_username,
            "--auth-password": self.auth_password,
        }

        # Not specifying parameters should make them null
        parameters = {key: value for key, value in parameters.items() if value is not None}

        process = Popen(
            [
                self.executable_path,
                *[
                    str(item)
                    for key, value in parameters.items()
                    for item in [key, value]
                ]
            ]
        )

        # Wait for launch
        sleep(0.5)

        try:
            yield
        finally:
            process.terminate()

    def tape_start(self):
        response = self.session.post(urljoin(self.base_url_control, "/api/tape/record"), timeout=self.timeout)
        assert response.json()["success"] == True

    def tape_get(self, tape_id: str | None = None) -> TapeSession:
        tape_response = self.session.post(
            urljoin(self.base_url_control, "/api/tape/retrieve"),
            json=dict(
                tapeID=tape_id,
            ),
            timeout=self.timeout
        )
        return TapeSession.from_server(tape_response.content)

    def tape_load(self, session: TapeSession):
        tape_response = self.session.post(
            urljoin(self.base_url_control, "/api/tape/load"),
            files={"file": session.to_server()},
            timeout=self.timeout,
        )
        assert tape_response.json()["success"] == True

    def tape_stop(self):
        response = self.session.post(urljoin(self.base_url_control, "/api/tape/stop"), timeout=self.timeout)
        assert response.json()["success"] == True

    def tape_clear(self, tape_id: str | None = None):
        response = self.session.post(
            urljoin(self.base_url_control, "/api/tape/clear"),
            json=dict(
                tapeID=tape_id,
            ),
            timeout=self.timeout,
        )
        assert response.json()["success"] == True

    def set_cache_mode(self, mode: CacheModeEnum):
        response = self.session.post(
            urljoin(self.base_url_control, "/api/cache/mode"),
            json=dict(
                mode=mode.value,
            )
        )
        assert response.json()["success"] == True

    def cache_clear(self):
        response = self.session.post(urljoin(self.base_url_control, "/api/cache/clear"), timeout=self.timeout)
        assert response.json()["success"] == True

    def dialer_load(self, dialers: list[DialerDefinition]):
        response = self.session.post(
            urljoin(self.base_url_control, "/api/dialer/load"),
            json=dict(
                definitions=[
                    {
                        "priority": dialer.priority,
                        "proxyServer": dialer.proxy.url if dialer.proxy is not None else None,
                        "proxyUsername": dialer.proxy.username if dialer.proxy is not None else None,
                        "proxyPassword": dialer.proxy.password if dialer.proxy is not None else None,
                        "requiresUrlRegex": dialer.request_requires.url_regex if dialer.request_requires is not None else None,
                        "requiresResourceTypes": dialer.request_requires.resource_types if dialer.request_requires is not None else None,
                    }
                    for dialer in dialers
                ],
            )
        )
        assert response.json()["success"] == True

    @property
    def executable_path(self) -> str:
        # Support statically and dynamically build libraries
        if (path := get_asset_path("grooveproxy")).exists():
            return str(path)

        wheel_extension = get_config_var("EXT_SUFFIX")
        if (path := get_asset_path(f"grooveproxy{wheel_extension}")).exists():
            return exit(path)

        raise ValueError("No groove executable file found")
