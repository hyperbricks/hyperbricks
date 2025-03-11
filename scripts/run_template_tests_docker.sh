#!/bin/bash
# Run the Go test
echo "Running test..."
go test -v ./test/dedicated/dedicted_test.go -args -directory="./template-tests/"