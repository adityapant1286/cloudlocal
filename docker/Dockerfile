
# --- Stage 1: Build a custom, minimal JRE ---
FROM azul/zulu-openjdk-debian:21-latest AS jre-builder

RUN apt-get update && apt-get install -y binutils

RUN $JAVA_HOME/bin/jlink \
    --add-modules java.base,java.logging,java.xml,java.naming,java.desktop,java.management,java.sql,jdk.unsupported \
    --strip-debug \
    --no-man-pages \
    --no-header-files \
    --compress zip-9 \
    --output /javaruntime

# --- Stage 2: Go Builder ---
FROM golang:1.25-bookworm AS go-builder
WORKDIR /app
COPY go.mod ./
# Only copy go.sum if it exists
COPY go.sum* ./
RUN go mod download
COPY . .
# This COPY captures the /ui folder for embedding
RUN CGO_ENABLED=0 GOOS=linux go build -o cloudlocal-edge main.go

# --- Stage 3: Extract Node.js Binary (Minimal) ---
FROM node:25-bookworm-slim AS node-extractor

# --- Stage 4: Extract Python Binary (Minimal) ---
FROM python:3.14-slim-bookworm AS python-extractor

# --- Stage 5: Final Slim Image ---
FROM debian:bookworm-slim

ARG LAMBDA_FUNCTIONS_DIRECTORY

ENV JAVA_HOME=/opt/java \
    PATH="/opt/java/bin:/usr/local/bin:$PATH" \
    APP_HOME=/opt/cloudlocal \
    LD_LIBRARY_PATH="/usr/local/lib:$LD_LIBRARY_PATH" \
    PYTHONUNBUFFERED=1


WORKDIR "$APP_HOME"

# Install base dependencies and DynamoDB
RUN apt-get update && apt-get install -y --no-install-recommends \
    wget unzip ca-certificates libsqlite3-0 libatomic1 \
    && wget -q https://d1ni2b6xgvw0s0.cloudfront.net/v2.x/dynamodb_local_latest.zip -O /tmp/db.zip \
#    && wget -q https://s3.us-west-2.amazonaws.com/dynamodb-local/dynamodb_local_latest.zip -O /tmp/db.zip \
    && mkdir -p "$APP_HOME/dynamodb_local_latest" \
    && unzip /tmp/db.zip -d "$APP_HOME/dynamodb_local_latest" \
    && find "$APP_HOME/dynamodb_local_latest/DynamoDBLocal_lib" -name "*.dll" -delete \
    && find "$APP_HOME/dynamodb_local_latest/DynamoDBLocal_lib" -name "*.dylib" -delete \
    && rm /tmp/db.zip \
    && apt-get purge -y wget unzip \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p "$APP_HOME/shims"

# 1. Copy JRE
COPY --from=jre-builder /javaruntime $JAVA_HOME

# 2. Copy Node.js (Just the binary and essential libs)
COPY --from=node-extractor /usr/local/bin/node /usr/local/bin/node

# 3. Copy Python (Minimal install)
COPY --from=python-extractor /usr/local/bin/python3.14 /usr/local/bin/python3
COPY --from=python-extractor /usr/local/lib/python3.14 /usr/local/lib/python3.14
COPY --from=python-extractor /usr/local/lib/libpython3.14.so.* /usr/local/lib/

# 4. Copy CloudLocal binaries
COPY ./docker/shims/node_shim.js $APP_HOME/shims/node_shim.js
COPY ./docker/shims/python_shim.py $APP_HOME/shims/python_shim.py
COPY --from=go-builder /app/cloudlocal-edge $APP_HOME/cloudlocal-edge
COPY ./docker/shell/main.sh ${APP_HOME}/main.sh

RUN chmod +x ${APP_HOME}/main.sh

EXPOSE 10050-10070

ENTRYPOINT ["/opt/cloudlocal/main.sh"]