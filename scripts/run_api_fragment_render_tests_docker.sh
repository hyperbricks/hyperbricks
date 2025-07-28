#!/bin/bash

# Start the Go server in the background
echo "Starting docker server..."
go run ./cmd/testing/main.go &
docker-compose -f ./test/dedicated/docker/docker-compose.yml up -d

# Give the server a moment to start
sleep 10

# Run the Go test
echo "Running docker API_RENDER and API_FRAGENT_RENDER test..."
go test -v ./test/dedicated/dedicted_test.go -args -directory="./api-tests/"

# Stop the server after the test
docker-compose -f ./test/dedicated/docker/docker-compose.yml down -v
pkill -f "/exe/main"