#! /bin/bash -e

# On OSX can be installed by `brew install coreutils`
scriptPath=$(realpath $0)
rootDirectory="$(dirname "$scriptPath")"

echo "Rebuilding groove proxy..."

# Clear out build files so we can more easily see build failures
rm -rf $rootDirectory/build
rm -rf $rootDirectory/groove-python/groove/assets/grooveproxy

# Build
mkdir -p $rootDirectory/build
(cd $rootDirectory/proxy && go build -o $rootDirectory/build)

# Manual Python install
cp $rootDirectory/build/grooveproxy $rootDirectory/groove-python/groove/assets/grooveproxy

# Manual Node install
cp $rootDirectory/build/grooveproxy $rootDirectory/groove-node/node_modules/.bin/grooveproxy

# Node build
(cd $rootDirectory/groove-node && npm run build)
