#! /bin/bash

#
# Global setup script for the project
#
set -e

# mitmproxy
echo "Setting up mitmproxy"
(cd proxy_benchmarks/assets/proxies/mitmproxy && ./setup.sh)

# node_http_proxy
echo "Setting up node_http_proxy"
(cd proxy_benchmarks/assets/proxies/node_http_proxy && ./setup.sh)

# gomitmproxy
echo "Setting up gomitmproxy"
(cd proxy_benchmarks/assets/proxies/gomitmproxy && ./setup.sh)
(cd proxy_benchmarks/assets/proxies/gomitmproxy-mimic && ./setup.sh)

# goproxy
echo "Setting up goproxy"
(cd proxy_benchmarks/assets/proxies/goproxy && ./setup.sh)
(cd proxy_benchmarks/assets/proxies/goproxy-mimic && ./setup.sh)

# martian
echo "Setting up martian"
(cd proxy_benchmarks/assets/proxies/martian && ./setup.sh)

# speed-test-server
echo "Setting up speed-test-server"
(cd proxy_benchmarks/assets/speed-test/server && ./setup.sh)
