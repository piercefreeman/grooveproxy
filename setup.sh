#
# Global setup script for the project
#

# mitmproxy
(cd proxy_benchmarks/assets/mitmproxy && ./setup.sh)

# node_http_proxy
(cd proxy_benchmarks/assets/node_http_proxy && npm install && npm run setup)

# gomitmproxy
(cd proxy_benchmarks/assets/gomitmproxy && go install && ./setup.sh)

# goproxy
(cd proxy_benchmarks/assets/goproxy && go install && ./setup.sh)

# speed-test-server
(cd speed-test-server && go install && ./setup.sh)
