#!/bin/bash

# Start the Go server in the background
echo "Starting token validation server..."
go run ./cmd/testing/main.go &

# Capture the server's process ID
SERVER_PID=$!

# Give the server a moment to start
sleep 2

# Run the test using curl or another Go test
echo "Running test..."
go test ./test/dedicated/dedicted_test.go -v

# Stop the server after the test
echo "Stopping server..."
pkill -f "/exe/main"