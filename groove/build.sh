#! /bin/bash -e

echo "Rebuilding groove proxy..."

# Clear out build files so we can more easily see build failures
rm -rf build
rm -rf ./groove-python/groove/assets/grooveproxy

# Build
mkdir -p build
(cd proxy && go build -o ../build)

# Python
rm -rf ./groove-python/groove/assets/ssl
cp ./build/grooveproxy ./groove-python/groove/assets/grooveproxy
cp -r ./proxy/ssl ./groove-python/groove/assets/ssl
