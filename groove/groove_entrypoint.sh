#!/bin/bash -e

if [ "$1" = "test-python" ]; then
    exec cd groove-python && poetry run pytest -s proxy_benchmarks/tests "${@:2}"
else
    exec "$@"
fi
