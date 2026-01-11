#!/bin/bash
# https://www.geeksforgeeks.org/linux-unix/if-command-in-linux-with-examples/

APP_HOME=/opt/cloudlocal
APP_DATA_HOME=/opt/cloudlocal-data
SERVICES="${ENABLED_SERVICES:-}"
SERVICES="${SERVICES,,}" # Lowercase for easier matching
SERVICES="${SERVICES// /}" # Remove all spaces for easier matching

PIDS=() # Array to keep track of all service PIDs

# Function to stop all background processes gracefully
cleanup() {
  echo -e "\n[Shutdown] Gracefully terminating Cloudlocal..."
    for pid in "${PIDS[@]}"; do
        kill -TERM "$pid" 2>/dev/null
    done
  wait
  echo "[Shutdown] All services of Cloudlocal cleaned up."
  exit 0
}

# Trap SIGINT (Ctrl+C) and SIGTERM (Docker stop)
trap 'cleanup' SIGINT SIGTERM

contains_service() {
  [[ ",$SERVICES," == *",${1,,},"* ]]
}

mkdir -p "$APP_DATA_HOME/logs"
mkdir -p "$APP_HOME/logs"

# --- Service 1: DynamoDB Local ---
dynamodb_service() {
  DYNAMODB_PORT=10051
  DYNAMO_DB_LOCAL="$APP_HOME/dynamodb_local_latest"

  DYNAMO_DB_PATH="$APP_DATA_HOME/dynamodb"

  mkdir -p "$DYNAMO_DB_PATH"

#  echo "Starting DynamoDB Local"

  # start DynamoDB Local
  java --enable-native-access=ALL-UNNAMED \
      -Djava.library.path="$DYNAMO_DB_LOCAL/DynamoDBLocal_lib" \
      -Dsqlite4java.library.path="$DYNAMO_DB_LIB" \
      -jar "$DYNAMO_DB_LOCAL/DynamoDBLocal.jar" \
      -dbPath "$DYNAMO_DB_PATH" \
      -port $DYNAMODB_PORT -sharedDb -disableTelemetry > "$APP_DATA_HOME/logs/dynamodb.log" 2>&1 &
#      | grep -vE "Initializing DynamoDB Local|Port:|InMemory:|Version:|DbPath:|SharedDb:|shouldDelayTransientStatuses:|CorsParams:" &

  PIDS+=($!) # Store the PID
}

# --- Go Edge Dispatcher (The "Brain") ---
edge_dispatcher() {
#  echo "Starting CloudLocal Edge Dispatcher on port 10050..."
  # We start this in the background just like others
  ./cloudlocal-edge > "$APP_HOME/logs/edge.log" 2>&1 &
  PIDS+=($!)
}

if contains_service "dynamodb"; then
  dynamodb_service
fi

if [ ${#PIDS[@]} -gt 0 ]; then
  sleep 2
  edge_dispatcher
fi

# --- Keep alive ---
if [ ${#PIDS[@]} -eq 0 ]; then
  echo "No services were enabled. Check ENABLED_SERVICES env var."
  exit 1
fi

echo "CloudLocal is up and running on port 10050..."
echo "Press Ctrl+C to shut down."

# Wait for all background processes.
wait