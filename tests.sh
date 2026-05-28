#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}"
RESULTS_FILE="${REPO_ROOT}/test-results.txt"

usage() {
    cat <<'USAGE'
Usage: ./tests.sh [--with-docs] [--with-plugins]

Runs the repository test suite. Documentation generation is skipped by default.

Options:
  --with-docs     Regenerate README, reference docs, and documentation test results.
  --with-plugins  Install root npm dependencies, rebuild local HyperBricks
                  plugins, and run plugin-backed runtime smoke tests.
  -h, --help      Show this help text.
USAGE
}

cd "${REPO_ROOT}"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
fi

if bash "${REPO_ROOT}/scripts/run_all_tests.sh" "$@" 2>&1 | tee "${RESULTS_FILE}"; then
    echo "All tests passed successfully."
    exit 0
else
    echo "Tests failed. See ${RESULTS_FILE} for details."
    exit 1
fi
