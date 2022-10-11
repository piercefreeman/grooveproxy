# mitmproxy will look for these specially named files
openssl genrsa -out mitmproxy-ca.key 2048
openssl req -new -x509 -key mitmproxy-ca.key -out mitmproxy-ca.crt -subj "/C=US/ST=CA/L=SF/O= /OU= /CN= /emailAddress= "

security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db ./mitmproxy-ca.crt
