#!/bin/bash

set -e

echo "Running docker CACHE AND PATH MARKER test..."

# Define an array of routes to check
ROUTES=("http://localhost:8085/test_001" "http://localhost:8085/test_002" "http://localhost:8085/test_003")  # Add more routes as needed

# Define the path to expected result files
EXPECTED_FILES=(
  "test/dedicated/cache-path-marker-tests/expected_result-001.html"
  "test/dedicated/cache-path-marker-tests/expected_result-002.html"
  "test/dedicated/cache-path-marker-tests/expected_result-003.html"
  # Add more expected files here
)

# Function to normalize the HTML content by removing extra whitespace
normalize_html() {
  echo "$1" | tr -s '[:space:]' ' ' | sed 's/[[:space:]]*$//g'
}

# Start hyperbricks in the background and capture its PID
echo "Starting hyperbricks..."
go run ./cmd/hyperbricks start -m markers-test -p 8085 > /tmp/hyperbricks.log 2>&1 &
PROCESS_PID=$!

# Function to kill the process if the script exits or fails
trap "kill -9 $PROCESS_PID > /dev/null 2>&1" EXIT

echo "Hyperbricks process started with PID $PROCESS_PID."

# Wait for hyperbricks to be fully ready
echo "Waiting for hyperbricks to be ready..."
sleep 2

# Flag to track test results
ALL_TESTS_PASSED=true

# Loop through all routes with their index
for idx in "${!ROUTES[@]}"; do
  ROUTE="${ROUTES[$idx]}"
  EXPECTED_FILE="${EXPECTED_FILES[$idx]}"  # Get the corresponding expected file by index

  echo "Testing route: $ROUTE"
  
  RESPONSE=$(curl -s "$ROUTE")

  # If still no response, print logs and exit
  if [ -z "$RESPONSE" ]; then
    echo "Failed to get a response from hyperbricks at $ROUTE."
    cat /tmp/hyperbricks.log
    exit 1
  fi

  # Read the expected result from the file
  EXPECTED_RESULT=$(cat "$EXPECTED_FILE")

  # Normalize both actual and expected responses
  EXPECTED_RESULT_NORMALIZED=$(normalize_html "$EXPECTED_RESULT")
  RESPONSE_NORMALIZED=$(normalize_html "$RESPONSE")

  # Perform the comparison between normalized actual and expected responses
  if [[ "$RESPONSE_NORMALIZED" == "$EXPECTED_RESULT_NORMALIZED" ]]; then
    echo "PASS: HTML structure matches the expected template from $EXPECTED_FILE."
  else
    echo "FAIL: HTML structure does not match the expected template from $EXPECTED_FILE."
    echo "Expected HTML structure (normalized):"
    echo "$EXPECTED_RESULT_NORMALIZED"
    echo "Actual HTML structure (normalized):"
    echo "$RESPONSE_NORMALIZED"
    ALL_TESTS_PASSED=false
  fi
done

# Kill hyperbricks processes on port 8085
echo "Killing hyperbricks processes on port 8085..."
ps aux | grep 'hyperbricks' | grep -v grep | awk '{print $2}' | xargs kill -9

echo "Waiting for exiting hyperbricks..."
sleep 1

# Check if the tests passed
if $ALL_TESTS_PASSED; then
  echo "All tests passed successfully!"
else
  echo "Some tests failed."
fi

# Check if port 8085 is in use
if lsof -i :8085 > /dev/null; then
  echo "Port 8085 is in use."
else
  echo "Everything cleaned"
fi
