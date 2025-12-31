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

# --- Stage 3: Final Slim Image ---
FROM debian:bookworm-slim

ENV JAVA_HOME=/opt/java \
    PATH="/opt/java/bin:$PATH" \
    APP_HOME=/opt/cloudlocal

WORKDIR "$APP_HOME"

# Combine everything into ONE layer to keep the image small
RUN apt-get update && apt-get install -y --no-install-recommends \
    wget unzip ca-certificates libsqlite3-0 \
    && wget -q https://s3.us-west-2.amazonaws.com/dynamodb-local/dynamodb_local_latest.zip -O /tmp/db.zip \
    && mkdir -p "$APP_HOME/dynamodb_local_latest" \
    && unzip /tmp/db.zip -d "$APP_HOME/dynamodb_local_latest" \
    && find "$APP_HOME/dynamodb_local_latest/DynamoDBLocal_lib" -name "*.dll" -delete \
    && find "$APP_HOME/dynamodb_local_latest/DynamoDBLocal_lib" -name "*.dylib" -delete \
    && rm /tmp/db.zip \
    && apt-get purge -y wget unzip \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

COPY --from=jre-builder /javaruntime $JAVA_HOME
COPY --from=go-builder /app/cloudlocal-edge $APP_HOME/cloudlocal-edge
COPY ./docker/shell/main.sh ${APP_HOME}/main.sh

RUN chmod +x ${APP_HOME}/main.sh

EXPOSE 10050-10070

ENTRYPOINT ["/opt/cloudlocal/main.sh"]