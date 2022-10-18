#! /bin/bash -e

echo "Rebuilding groove proxy..."

# Clear out build files so we can more easily see build failures
rm -rf build
rm -rf ./groove-python/groove/assets/grooveproxy

# Build
mkdir -p build
(cd proxy && go build -o ../build)

# Manual Python install
cp ./build/grooveproxy ./groove-python/groove/assets/grooveproxy

# Manual Node install
cp ./build/grooveproxy ./groove-node/node_modules/.bin/grooveproxy
