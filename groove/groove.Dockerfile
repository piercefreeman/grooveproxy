FROM ubuntu:22.04

ENV DOCKER "1"
ENV PYTHONUNBUFFERED "1"
ENV NODE_VERSION v16.14.0
ENV NVM_DIR /usr/local/nvm

RUN apt-get -y update \
    && apt-get -y install python3 python3.10-venv curl gcc python3-dev sudo ca-certificates golang-go git

# Install node. We need to source the nvm executable via `. nvm.sh` to allow the script to work inside sh, which
# is the default shell during docker build
RUN mkdir -p $NVM_DIR \
    && curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.1/install.sh | bash \
    && . $NVM_DIR/nvm.sh \
    && nvm install $NODE_VERSION

RUN curl -sSL https://install.python-poetry.org | python3 -

# Required to add poetry and node executables to the path
ENV PATH="/root/.local/bin:/$NVM_DIR/versions/node/$NODE_VERSION/bin:$PATH"

WORKDIR /app

# Install benchmarking dependencies
ADD groove-python/poetry.lock groove-python/poetry.lock
ADD groove-python/pyproject.toml groove-python/pyproject.toml
RUN cd groove-python && poetry install --no-root

ADD . /app
ADD ./groove_entrypoint.sh /app/benchmark_entrypoint.sh

# Mount the scripts, don't perform any additional installation
RUN cd groove-python && poetry install --no-interaction

# Install the certificate management tools that Chromium uses on Linux
# This is required to add our custom certificates
# https://chromium.googlesource.com/chromium/src/+/master/docs/linux/cert_management.md
RUN apt-get install -y libnss3-tools
RUN mkdir -p $HOME/.pki/nssdb

# Install the dependent packages
RUN ./setup.sh
RUN ./build.sh

# Install the root certificates
RUN cd proxy && go run . install-ca

RUN cd groove-python && poetry run playwright install-deps chromium
RUN cd groove-python && poetry run playwright install chromium

ENTRYPOINT [ "/app/groove_entrypoint.sh" ]
