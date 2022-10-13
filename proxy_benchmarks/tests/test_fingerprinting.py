#from click.testing import CliRunner
from proxy_benchmarks.cli.fingerprinting import compare_dynamic_raw
from proxy_benchmarks.requests import ChromeRequest
from proxy_benchmarks.proxies.goproxy import GoProxy
from proxy_benchmarks.enums import MimicTypeEnum

def test_fingerprinting(cli_object):
    compare_dynamic_raw(cli_object, ChromeRequest(headless=True), [GoProxy(MimicTypeEnum.STANDARD)])
