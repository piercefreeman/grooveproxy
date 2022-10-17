#!/bin/bash -e

if [ "$1" = "test" ]; then
    exec poetry run pytest -s proxy_benchmarks/tests "${@:2}"
else
    exec "$@"
fi
