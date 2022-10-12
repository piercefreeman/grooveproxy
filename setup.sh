#
# Global setup script for the project
#
set -e

# mitmproxy
(cd proxy_benchmarks/assets/proxies/mitmproxy && ./setup.sh)

# node_http_proxy
(cd proxy_benchmarks/assets/proxies/node_http_proxy && npm install && npm run setup)

# gomitmproxy
(cd proxy_benchmarks/assets/proxies/gomitmproxy && go install && ./setup.sh)
(cd proxy_benchmarks/assets/proxies/gomitmproxy-mimic && go install && ./setup.sh)

# goproxy
(cd proxy_benchmarks/assets/proxies/goproxy && go install && ./setup.sh)
(cd proxy_benchmarks/assets/proxies/goproxy-mimic && go install && ./setup.sh)

# martian
(cd proxy_benchmarks/assets/proxies/martian && go install && ./setup.sh)

# speed-test-server
(cd proxy_benchmarks/assets/speed-test/server && go install && ./setup.sh)
