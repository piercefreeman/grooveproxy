import pytest

from groove.proxy import Groove


# We want this to recreate by default on every unit test to clear the state
@pytest.fixture(scope="function")
def proxy():
    proxy = Groove()
    with proxy.launch():
        yield proxy
