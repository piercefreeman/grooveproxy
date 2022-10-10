# Create a custom openssl config that sets the subject of the certificate to localhost
cat /etc/ssl/openssl.cnf > openssl_config.conf
echo '\n[SAN]\nsubjectAltName=DNS:localhost,IP:127.0.0.1' >> openssl_config.conf

# https://serverfault.com/questions/880804/can-not-get-rid-of-neterr-cert-common-name-invalid-error-in-chrome-with-self
openssl genrsa -out cert.key 2048
openssl req \
    -newkey rsa:2048 \
    -x509 \
    -nodes \
    -keyout cert.key \
    -new \
    -out cert.crt \
    -subj /CN=Hostname \
    -reqexts SAN \
    -extensions SAN \
    -config openssl_config.conf \
    -sha256 \
    -days 3650

security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db ./cert.crt

rm openssl_config.conf
