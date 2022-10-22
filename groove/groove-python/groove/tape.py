from base64 import b64decode, b64encode
from gzip import compress, decompress
from json import dumps, loads

from pydantic import validator

from groove.models import GrooveModelBase


class TapeRequest(GrooveModelBase):
    url: str
    method: str
    headers: dict[str, list[str]]
    body: bytes

    @validator("body")
    def validate_body(cls, value):
        return b64decode(value)


class TapeResponse(GrooveModelBase):
    status: int
    headers: dict[str, list[str]]
    body: bytes

    @validator("body")
    def validate_body(cls, value):
        return b64decode(value)


class TapeRecord(GrooveModelBase):
    request: TapeRequest
    response: TapeResponse

    class Config:
        json_encoders = {
            # Assume that body bytes should always be encoded as base64 strings
            bytes: lambda value: b64encode(value).decode(),
        }


class TapeSession(GrooveModelBase):
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