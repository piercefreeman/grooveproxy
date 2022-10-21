from enum import Enum

class CacheModeEnum(Enum):
    # Ensure enum values are aligned with the cache.go definitions
    OFF = 0
    STANDARD = 1
    AGGRESSIVE = 2
