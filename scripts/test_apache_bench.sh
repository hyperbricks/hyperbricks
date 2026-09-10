#!/usr/bin/env bash
set -euo pipefail

TARGET_URL="${1:-${BASE_URL:-http://127.0.0.1:8080/}}"

echo "Apache Benchmarking: ${TARGET_URL}"
ab -n 10000 -c 150 "${TARGET_URL}"
