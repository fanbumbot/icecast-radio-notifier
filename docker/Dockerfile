FROM python:3.12

WORKDIR /app

COPY requirements.txt requirements.txt

RUN \
    apt-get update && \
    apt-get install python3 -y && \
    apt-get install python3-pip -y && \
    apt-get install python3-venv -y && \
    rm -rf /var/lib/apt/lists/* && \
    python3 -m venv /app/.venv && \
    . /app/.venv/bin/activate && \
    pip install --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

COPY ./src /app/src
COPY ./.gitignore /app/.gitignore
COPY ./.dockerignore /app/.dockerignore
COPY ./docker /app/docker

# Setup entrypoint
COPY ./docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh
ENTRYPOINT ["/app/entrypoint.sh"]