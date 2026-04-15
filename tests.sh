#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}"
RESULTS_FILE="${REPO_ROOT}/test-results.txt"

cd "${REPO_ROOT}"

if "${REPO_ROOT}/scripts/run_all_tests.sh" 2>&1 | tee "${RESULTS_FILE}"; then
    echo "All tests passed successfully."
    exit 0
else
    echo "Tests failed. See ${RESULTS_FILE} for details."
    exit 1
fi
