#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

echo "Running all tests..."

go test ./cmd/hyperbricks/...
go test ./test/main -v
bash "${SCRIPT_DIR}/run_api_fragment_render_tests_docker.sh"
bash "${SCRIPT_DIR}/run_template_tests.sh"
bash "${SCRIPT_DIR}/run_marker_tests.sh"
bash "${SCRIPT_DIR}/build_docs.sh"
bash "${SCRIPT_DIR}/test_headers_module.sh"
