openssl genrsa -out ca.key 2048
openssl req -new -x509 -key ca.key -out ca.crt -subj "/C=US/ST=CA/L=SF/O= /OU= /CN= /emailAddress= "

security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db ./ca.crt
