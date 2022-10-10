from abc import ABC, abstractmethod
from contextlib import contextmanager


class ProxyBase(ABC):
    def __init__(self, port=8080):
        self.port = port

    @abstractmethod
    @contextmanager
    def launch(self):
        pass

    @abstractmethod
    def __repr__(self) -> str:
        return super().__repr__()
