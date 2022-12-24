from subprocess import run

from groove.assets import get_asset_path


def install_ca():
    run([
        str(get_asset_path("grooveproxy")),
        "install-ca",
    ])
