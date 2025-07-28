#!/bin/bash
# Run the Go test
echo "Running all test..."

go test ./test/main -v
scripts/run_api_fragment_render_tests_docker.sh
scripts/run_template_tests.sh
scripts/run_marker_tests.sh
scripts/build_docs.sh