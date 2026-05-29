#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

echo "Running YAML dedicated template tests..."
go test -v ./test/dedicated -run '^Test_All_Dedicated_YAML_Tests$' -args -directory="./yaml-template-tests/"
