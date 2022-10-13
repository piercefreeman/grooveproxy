#! /bin/bash
set -e

npm install
npm run setup

if [ "$(uname)" == "Darwin" ]; then
    # Mac OS X platform
    sudo security add-trusted-cert -d -p ssl -p basic -k /Library/Keychains/System.keychain .http-mitm-proxy/certs/ca.pem
elif [ "$(expr substr $(uname -s) 1 5)" == "Linux" ]; then
    # GNU/Linux
    cp .http-mitm-proxy/certs/ca.pem /usr/local/share/ca-certificates/http-mitm-proxy-ca.pem
    sudo update-ca-certificates
fi
