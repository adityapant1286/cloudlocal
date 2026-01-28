#!/bin/bash
# https://www.geeksforgeeks.org/linux-unix/if-command-in-linux-with-examples/

APP_HOME=/opt/cloudlocal
APP_DATA_HOME=/opt/cloudlocal-data/cloudlocal
SERVICES="${ENABLED_SERVICES:-}"
SERVICES="${SERVICES,,}" # Lowercase for easier matching
SERVICES="${SERVICES// /}" # Remove all spaces for easier matching

PIDS=() # Array to keep track of all service PIDs
RED_L='\033[1;31m'
GREEN_L='\033[1;32m'
GRAY_D='\033[1;30m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to stop all background processes gracefully
cleanup() {
  echo -e "\n[${YELLOW}Shutdown${NC}] Gracefully terminating Cloudlocal..."
    for pid in "${PIDS[@]}"; do
        kill -TERM "$pid" 2>/dev/null
    done
  wait
  echo "[${GRAY_D}Shutdown${NC}] All services of Cloudlocal cleaned up."
  exit 0
}

# Trap SIGINT (Ctrl+C) and SIGTERM (Docker stop)
trap 'cleanup' SIGINT SIGTERM

contains_service() {
  [[ ",$SERVICES," == *",${1,,},"* ]]
}

mkdir -p "$APP_DATA_HOME/logs"

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
  if [ "${CLOUDWATCH_CONSOLE_LOG}" == "true" ]; then
    ./cloudlocal-edge 2>&1 | tee >(sed -u 's/\x1b\[[0-9;]*m//g' >> "$APP_DATA_HOME/logs/edge.log") &
  else
    ./cloudlocal-edge > "$APP_DATA_HOME/logs/edge.log" 2>&1 &
  fi


  PIDS+=($!)
}

check_supported_runtime() {

  if command -v java >/dev/null; then
      echo -e "[${GREEN_L}OK${NC}] Java: $(java -version 2>&1 | head -n 1)"
  else
      echo -e "[${RED_L}FAIL${NC}] Java not found"
  fi

  # Check Node
  if command -v node >/dev/null; then
      echo -e "[${GREEN_L}OK${NC}] Node: $(node -v)"
  else
      echo -e "[${RED_L}FAIL${NC}] Node not found"
  fi

  # Check Python
  if command -v python3 >/dev/null; then
      # This also checks if the shared library (.so) is linked correctly
      echo -e "[${GREEN_L}OK${NC}] Python: $(python3 --version)"
  else
      echo -e "[${RED_L}FAIL${NC}] Python not found or library link broken"
  fi

  # Check Go (Your CloudLocal binary)
  if [ -f "/opt/cloudlocal/cloudlocal-edge" ]; then
      echo -e "[${GREEN_L}OK${NC}] CloudLocal Edge Binary found"
  else
      echo -e "[${RED_L}FAIL${NC}] CloudLocal Edge Binary missing"
  fi

}

if contains_service "dynamodb"; then
  dynamodb_service
fi

if contains_service "lambda"; then
  check_supported_runtime
fi

if [ ${#PIDS[@]} -gt 0 ]; then
  sleep 2
  edge_dispatcher
fi

# --- Keep alive ---
if [ ${#PIDS[@]} -eq 0 ]; then
  echo -e "No services were enabled. Check ENABLED_SERVICES env var."
  exit 1
fi

echo -e "CloudLocal is up and running on port 10050..."
echo -e "Press Ctrl+C to shut down."

# Wait for all background processes.
wait