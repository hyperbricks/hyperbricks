#!/bin/bash
# Start the Go server in the background
echo "Starting docker server..."
go run ./cmd/testing/main.go &

echo "Running test..."
go test ./test/dedicated/dedicted_test.go -v


pkill -f "/exe/main"
