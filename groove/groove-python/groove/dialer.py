from groove.models import GrooveModelBase


class RequestRequiresDefinition(GrooveModelBase):
    url_regex: str
    resource_types: list[str]


class ProxyDefinition(GrooveModelBase):
    url: str
    username: str | None = None
    password: str | None = None


class DialerDefinition(GrooveModelBase):
    priority: int
    proxy: ProxyDefinition | None = None
    request_requires: RequestRequiresDefinition | None = None


class DefaultInternetDialer(DialerDefinition):
    """
    Proxy all requests to the open internet, with low priority
    """
    def __init__(self):
        super().__init__(
            priority=1,
            proxy=None,
            request_requires=None,
        )

class DefaultLocalPassthroughDialer(DialerDefinition):
    """
    Proxy generally static assets to the open internet, with high priority
    """
    def __init__(self):
        super().__init__(
            priority=1000,
            proxy=None,
            request_requires=RequestRequiresDefinition(
                url_regex=".*?.(?:txt|json|css|less|js|mjs|cjs|gif|ico|jpe?g|svg|png|webp|mkv|mp4|mpe?g|webm|eot|ttf|woff2?)",
                resource_types=["script", "image", "stylesheet", "media", "font"],
            ),
        )
