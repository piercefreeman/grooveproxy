#! /bin/bash
set -e

go install

# Create a custom openssl config that sets the subject of the certificate to localhost
if [ "$(uname)" == "Darwin" ]; then
    # Mac OS X platform
    FILE=/etc/ssl/openssl.cnf
elif [ "$(expr substr $(uname -s) 1 5)" == "Linux" ]; then
    # GNU/Linux
    FILE=/usr/lib/ssl/openssl.cnf
fi

if [ -f "$FILE" ]; then
    echo "$FILE exists."
else 
    echo "$FILE does not exist."
    exit 1
fi

cat $FILE > openssl_config.conf

# Use printf instead of echo since linux doesn't render \n properly
printf '\n[SAN]\nsubjectAltName=DNS:localhost,IP:127.0.0.1,IP:127.0.0.2' >> openssl_config.conf

cat openssl_config.conf

# https://serverfault.com/questions/880804/can-not-get-rid-of-neterr-cert-common-name-invalid-error-in-chrome-with-self
openssl genrsa -out cert.key 2048
openssl req \
    -newkey rsa:2048 \
    -x509 \
    -nodes \
    -keyout cert.key \
    -new \
    -out cert.crt \
    -subj /CN=SpeedTestServer \
    -reqexts SAN \
    -extensions SAN \
    -config openssl_config.conf \
    -sha256 \
    -days 3650

if [ "$(uname)" == "Darwin" ]; then
    # Mac OS X platform
    sudo security add-trusted-cert -d -p ssl -p basic -k /Library/Keychains/System.keychain ./cert.crt
elif [ "$(expr substr $(uname -s) 1 5)" == "Linux" ]; then
    # GNU/Linux
    cp ./cert.crt /usr/local/share/ca-certificates/speed-test-server.crt
    sudo update-ca-certificates
    #certutil -A -n "speed-test-server" -d ~/.pki/nssdb -t C,, -a -i /usr/local/share/ca-certificates/speed-test-server.crt
    certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n "speed-test-server" -i /usr/local/share/ca-certificates/speed-test-server.crt
fi

rm openssl_config.conf

mkdir -p ssl
cp cert.key ssl/cert.key
cp cert.crt ssl/cert.crt
