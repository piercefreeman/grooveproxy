#!/bin/bash -e

if [ "$1" = "test-python" ]; then
    cd groove-python && exec poetry run pytest -s groove/tests "${@:2}"
elif [ "$1" = "deploy-python" ]; then
    cp -r proxy groove-python
    cd groove-python
    poetry build
    poetry publish --username $PYPI_USERNAME --password $PYPI_PASSWORD
else
    exec "$@"
fi
