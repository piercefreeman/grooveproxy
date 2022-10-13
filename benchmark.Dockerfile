FROM ubuntu:22.04

RUN apt-get -y update \
    && apt-get -y install python3 python3.10-venv curl gcc python3-dev sudo ca-certificates tcpdump golang-go git lsof

RUN mkdir -p /usr/local/nvm/
ENV NVM_DIR /usr/local/nvm
ENV NODE_VERSION v16.14.0

# Required to add poetry and node executables to the path
ENV PATH="/root/.local/bin:/usr/local/nvm/versions/node/$NODE_VERSION/bin:$PATH"

# Install node. We need to source the nvm executable via `. nvm.sh` to allow the script to work inside sh, which
# is the default shell during docker build
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.1/install.sh | bash \
    && . $NVM_DIR/nvm.sh \
    && nvm install $NODE_VERSION

RUN curl -sSL https://install.python-poetry.org | python3 -

WORKDIR /app

# Install benchmarking dependencies
ADD poetry.lock poetry.lock
ADD pyproject.toml pyproject.toml
RUN poetry install --no-root

ADD . /app

# Mount the scripts, don't perform any additional installation
RUN poetry install --no-interaction

# Install the dependent packages and root certificates
RUN ./setup.sh

RUN poetry run playwright install-deps chromium
RUN poetry run playwright install chromium

#RUN useradd -m docker && echo "docker:docker" | chpasswd && adduser docker sudo
#RUN adduser docker && usermod -aG sudo docker

#USER docker

ENV PYTHONUNBUFFERED "1"
