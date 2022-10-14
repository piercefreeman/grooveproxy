#! /bin/bash
set -e

openssl genrsa -out mitmproxy-ca.key 2048
openssl req -new -x509 -key mitmproxy-ca.key -out mitmproxy-ca.crt -subj "/C=US/ST=CA/L= /O= /OU= /CN=MitmProxy/emailAddress= "

# mitmproxy will look for a consolidate pem root file
cat mitmproxy-ca.key mitmproxy-ca.crt > mitmproxy-ca.pem

if [ "$(uname)" == "Darwin" ]; then
    # Mac OS X platform
    sudo security add-trusted-cert -d -p ssl -p basic -k /Library/Keychains/System.keychain mitmproxy-ca.crt
elif [ "$(expr substr $(uname -s) 1 5)" == "Linux" ]; then
    # GNU/Linux
    cp ./mitmproxy-ca.crt /usr/local/share/ca-certificates/mitmproxy-ca.crt
    sudo update-ca-certificates
    certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "mitmproxy" -i /usr/local/share/ca-certificates/mitmproxy-ca.crt
fi

mkdir -p ssl
cp mitmproxy-ca.key ssl/mitmproxy-ca.key
cp mitmproxy-ca.crt ssl/mitmproxy-ca.crt
cp mitmproxy-ca.pem ssl/mitmproxy-ca.pem
