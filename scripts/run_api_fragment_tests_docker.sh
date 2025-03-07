#!/bin/bash

# Start the Go server in the background
echo "Starting docker server..."
go run ./cmd/testing/main.go &
docker-compose -f ./test/dedicated/docker/docker-compose.yml up -d

# Give the server a moment to start
sleep 10

# Run the Go test
echo "Running test..."
go test ./test/dedicated/dedicted_test.go -v

# Stop the server after the test
docker-compose -f ./test/dedicated/docker/docker-compose.yml down -v
pkill -f "/exe/main"