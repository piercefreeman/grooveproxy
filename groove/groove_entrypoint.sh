#!/bin/bash -e

if [ "$1" = "test-python" ]; then
    cd groove-python && exec poetry run pytest -s groove/tests "${@:2}"
elif [ "$1" = "test-node" ]; then
    cd groove-node && exec npm run test "${@:2}"
elif [ "$1" = "test-go" ]; then
    cd proxy && exec go test "${@:2}"
else
    exec "$@"
fi
