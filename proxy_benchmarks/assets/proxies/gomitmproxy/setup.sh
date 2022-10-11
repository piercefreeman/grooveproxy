set -e

openssl genrsa -out ca.key 2048
openssl req -new -x509 -key ca.key -out ca.crt -subj "/C=US/ST=CA/L= /O= /OU= /CN=GoMitmProxy/emailAddress= "

sudo security add-trusted-cert -d -p ssl -p basic -k /Library/Keychains/System.keychain ./ca.crt
