#!/bin/bash -e

if [ "$1" = "test" ]; then
    exec poetry run pytest -s groovy/tests "${@:2}"
else
    exec "$@"
fi
