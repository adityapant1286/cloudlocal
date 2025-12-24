#!/bin/bash
ids=$(docker ps -a -q --filter "name=cloudlocal")
if [ -n "$ids" ]; then
  docker rm -f "$ids"
fi

# This stops old containers, builds if needed, and starts up
docker-compose -f ./docker/compose.yaml --project-directory ./ up --build

# Clean up the orphaned image left behind by the build
docker image prune -f