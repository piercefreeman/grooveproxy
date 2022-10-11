set -e

openssl genrsa -out mitmproxy-ca.key 2048
openssl req -new -x509 -key mitmproxy-ca.key -out mitmproxy-ca.crt -subj "/C=US/ST=CA/L= /O= /OU= /CN=MitmProxy/emailAddress= "

# mitmproxy will look for a consolidate pem root file
cat mitmproxy-ca.key mitmproxy-ca.crt > mitmproxy-ca.pem

#security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db ./mitmproxy-ca.crt
#sudo security add-trusted-cert -d -p ssl -p basic -k /Library/Keychains/System.keychain mitmproxy-ca-cert.pem
sudo security add-trusted-cert -d -p ssl -p basic -k /Library/Keychains/System.keychain mitmproxy-ca.crt
