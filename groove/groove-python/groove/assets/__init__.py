from pathlib import Path

from importlib.resources import files


def get_asset_path(asset_name: str) -> Path:
    return Path(files(__name__) / asset_name)
