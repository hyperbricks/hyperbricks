#!/bin/bash
# Run the Go test
echo "Running template test..."
go test ./test/dedicated/dedicted_test.go -args -directory="./template-tests/"