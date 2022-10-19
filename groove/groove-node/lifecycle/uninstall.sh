#! /bin/bash -e

npm_bin=$(npm bin)

rm $npm_bin/grooveproxy

echo "Uninstall completed."
