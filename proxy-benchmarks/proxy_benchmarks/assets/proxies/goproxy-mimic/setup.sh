#! /bin/bash
set -e

go install

openssl genrsa -out ca.key 2048
openssl req -new -x509 -key ca.key -out ca.crt -subj "/C=US/ST=CA/L= /O= /OU= /CN=GoProxy/emailAddress= "

if [ "$(uname)" == "Darwin" ]; then
    # Mac OS X platform
    sudo security add-trusted-cert -d -p ssl -p basic -k /Library/Keychains/System.keychain ./ca.crt
elif [ "$(expr substr $(uname -s) 1 5)" == "Linux" ]; then
    # GNU/Linux
    cp ./ca.crt /usr/local/share/ca-certificates/goproxy-mimic-ca.crt
    sudo update-ca-certificates
    certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "goproxy-mimic" -i /usr/local/share/ca-certificates/goproxy-mimic-ca.crt
fi

mkdir -p ssl
cp ca.crt ssl/ca.crt
cp ca.key ssl/ca.key
