from types import NotImplementedType
from pydantic import BaseModel, validator
from gzip import compress, decompress
from json import loads, dumps
from base64 import b64decode, b64encode
from requests import Session
from urllib.parse import urljoin
from enum import Enum


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
    def __init__(self, command_timeout=5):
        self.session = Session()
        self.base_url_proxy = "http://localhost:6010"
        self.base_url_control = "http://localhost:5010"
        self.timeout = command_timeout

    def launch(self):
        raise NotImplementedType

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
