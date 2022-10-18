#! /bin/bash -e

# Consider providing prebuilt go executables and dynamically downloading like:
# https://github.com/sanathkr/go-npm
current_directory=$(pwd)
tmp_directory=$(mktemp -d)
npm_bin=$(npm bin)

mkdir -p $npm_bin

echo "Building in $tmp_directory..."

cd $tmp_directory

git clone https://github.com/piercefreeman/grooveproxy.git
cd grooveproxy/groove/proxy
go build -o $npm_bin/grooveproxy

echo "Build and npm install complete."

cd $current_directory
