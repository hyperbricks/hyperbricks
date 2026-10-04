#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
LIFECYCLE_TMP="$(mktemp -d "${TMPDIR:-/tmp}/hyperbricks-hooks.XXXXXX")"
trap 'rm -rf -- "${LIFECYCLE_TMP}"' EXIT

cd "${REPO_ROOT}"
go build -o "${LIFECYCLE_TMP}/hyperbricks-hooks" ./cmd/hyperbricks
python3 modules/development-hooks-demo/tests/smoke.py \
  --binary "${LIFECYCLE_TMP}/hyperbricks-hooks"
