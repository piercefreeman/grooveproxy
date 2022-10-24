from enum import Enum


class CacheModeEnum(Enum):
    # Ensure enum values are aligned with the cache.go definitions
    OFF = 0
    STANDARD = 1
    AGGRESSIVE_GET = 2
    AGGRESSIVE = 3
