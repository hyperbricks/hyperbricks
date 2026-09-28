#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
WITH_DOCS=false
WITH_PLUGINS=false
WITH_MODULES=false
WITH_SCREENSHOTS=false

announce() {
  printf '\n------------------------------------------------------------------------\n%s\n' "$1"
}

usage() {
  cat <<'USAGE'
Usage: scripts/run_all_tests.sh [--with-docs] [--with-plugins] [--with-modules]
                                [--with-screenshots]

Runs the repository test suite. Documentation generation is skipped by default.

Options:
  --with-docs     Regenerate README, reference docs, and documentation test results.
  --with-plugins  Install root npm dependencies, rebuild local HyperBricks
                  plugins, and run plugin-backed runtime smoke tests.
  --with-modules  Start documented modules and run their HTTP smoke checks.
  --with-screenshots
                  Run the Playwright module screenshot checks (implies
                  --with-modules; requires npm dependencies and Chromium).
  -h, --help      Show this help text.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-docs)
      WITH_DOCS=true
      ;;
    --with-plugins)
      WITH_PLUGINS=true
      ;;
    --with-modules)
      WITH_MODULES=true
      ;;
    --with-screenshots)
      WITH_SCREENSHOTS=true
      WITH_MODULES=true
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

announce "Running all tests..."

if [[ "${WITH_DOCS}" == "true" ]]; then
  echo "Regenerating documentation before tests..."
  bash "${SCRIPT_DIR}/build_docs.sh"
else
  echo "Skipping documentation generation. Pass --with-docs to regenerate docs before tests."
fi

if [[ "${WITH_PLUGINS}" == "true" ]]; then
  echo "Installing root npm dependencies for plugin-backed assets..."
  npm i
  echo "Building local HyperBricks plugins..."
  bash "${SCRIPT_DIR}/plugins/build_hyperbricks_plugins.sh"
  announce "Running plugin-backed runtime smoke tests..."
  bash "${SCRIPT_DIR}/plugins/test_hyperbricks_patterns_plugins.sh"
  announce "Running the project lifecycle integration fixture..."
  python3 "${SCRIPT_DIR}/test_project_lifecycle.py" --with-plugin
else
  echo "Skipping plugin rebuild. Pass --with-plugins to rebuild plugins before tests."
fi

announce "Running go vet..."
go vet ./...

announce "Running Go package tests, including test/dedicated..."
go test ./...

announce "Running Docker-backed dedicated API render tests..."
bash "${SCRIPT_DIR}/run_api_fragment_render_tests_docker.sh"

announce "Running dedicated template tests..."
bash "${SCRIPT_DIR}/run_template_tests.sh"

announce "Running marker tests..."
bash "${SCRIPT_DIR}/run_marker_tests.sh"

announce "Running header module tests..."
bash "${SCRIPT_DIR}/test_headers_module.sh"

if [[ "${WITH_MODULES}" == "true" ]]; then
  announce "Running documented module HTTP smoke tests..."
  python3 "${SCRIPT_DIR}/test_modules.py"
else
  echo "Skipping documented module checks. Pass --with-modules to run them."
fi

if [[ "${WITH_SCREENSHOTS}" == "true" ]]; then
  echo "Building plugins required by visual module checks..."
  bash "${SCRIPT_DIR}/plugins/build_hyperbricks_plugins.sh"
  announce "Running documented module screenshot checks..."
  npm run test:module-screenshots
else
  echo "Skipping module screenshots. Pass --with-screenshots to run them."
fi
