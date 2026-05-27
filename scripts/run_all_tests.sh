#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
WITH_DOCS=false

usage() {
  cat <<'USAGE'
Usage: scripts/run_all_tests.sh [--with-docs]

Runs the repository test suite. Documentation generation is skipped by default.

Options:
  --with-docs  Regenerate README, reference docs, and documentation test results.
  -h, --help   Show this help text.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-docs)
      WITH_DOCS=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

cd "${REPO_ROOT}"

echo "Running all tests..."

echo "Running go vet..."
go vet ./...

echo "Running Go package tests, including test/dedicated..."
go test ./...

echo "Running Docker-backed dedicated API render tests..."
bash "${SCRIPT_DIR}/run_api_fragment_render_tests_docker.sh"

echo "Running dedicated template tests..."
bash "${SCRIPT_DIR}/run_template_tests.sh"

echo "Running marker tests..."
bash "${SCRIPT_DIR}/run_marker_tests.sh"

if [[ "${WITH_DOCS}" == "true" ]]; then
  echo "Regenerating documentation..."
  bash "${SCRIPT_DIR}/build_docs.sh"
else
  echo "Skipping documentation generation. Pass --with-docs to regenerate docs."
fi

echo "Running header module tests..."
bash "${SCRIPT_DIR}/test_headers_module.sh"
