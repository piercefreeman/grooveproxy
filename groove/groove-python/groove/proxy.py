from base64 import b64decode, b64encode
from contextlib import contextmanager
from enum import Enum
from gzip import compress, decompress
from json import dumps, loads
from subprocess import Popen
from sysconfig import get_config_var
from time import sleep
from urllib.parse import urljoin

from pydantic import BaseModel, validator
from requests import Session

from groove.assets import get_asset_path


class CacheModeEnum(Enum):
    # Ensure enum values are aligned with the cache.go definitions
    OFF = 0
    STANDARD = 1
    AGGRESSIVE = 2


class TapeRequest(BaseModel):
    url: str
    method: str
    headers: dict[str, list[str]]
    body: bytes

    @validator("body")
    def validate_body(cls, value):
        return b64decode(value)


class TapeResponse(BaseModel):
    status: int
    headers: dict[str, list[str]]
    body: bytes

    @validator("body")
    def validate_body(cls, value):
        return b64decode(value)


class TapeRecord(BaseModel):
    request: TapeRequest
    response: TapeResponse

    class Config:
        json_encoders = {
            # Assume that body bytes should always be encoded as base64 strings
            bytes: lambda value: b64encode(value).decode(),
        }


class TapeSession(BaseModel):
    records: list[TapeRecord]

    @classmethod
    def from_server(cls, data: bytes):
        raw_records = loads(decompress(data))
        return cls(records=raw_records)

    def to_server(self) -> bytes:
        return compress(
            dumps([
                # json_encoders doesn't operate on list items, must iterate manually
                # https://github.com/pydantic/pydantic/issues/4085
                loads(record.json())
                for record in self.records
            ]).encode()
        )


class Groove:
    def __init__(
        self,
        command_timeout: int = 5,
        port: int = 6010,
        control_port: int = 6011,
        proxy_server: str | None = None,
        proxy_username: str | None = None,
        proxy_password: str | None = None,
        auth_username: str | None = None,
        auth_password: str | None = None,
    ):
        self.session = Session()

        self.port = port
        self.control_port = control_port
        self.proxy_server = proxy_server
        self.proxy_username = proxy_username
        self.proxy_password = proxy_password
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
            "--proxy-server": self.proxy_server,
            "--proxy-username": self.proxy_username,
            "--proxy-password": self.proxy_password,
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

    def tape_get(self) -> TapeSession:
        tape_response = self.session.post(urljoin(self.base_url_control, "/api/tape/retrieve"), timeout=self.timeout)
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

    def set_cache_mode(self, mode: CacheModeEnum):
        response = self.session.post(
            urljoin(self.base_url_control, "/api/cache/mode"),
            json=dict(
                mode=mode.value,
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
