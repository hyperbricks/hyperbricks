#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
RESULTS_FILE="${REPO_ROOT}/test/docs/documentation_test_results.txt"

cd "${REPO_ROOT}"

version="$(tr -d '\n' < ./assets/version.md)"
buildtime="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

go test ./test/docs -run '^Test_TestAndDocumentationRender$' -v \
  -args -update-docs -version="${version}" -buildtime="${buildtime}" \
  > "${RESULTS_FILE}"

matches=$(grep -iF "PASS:" "${RESULTS_FILE}" || true)

if [ -z "$matches" ]; then
    pass_num_matches=0
else
    pass_num_matches=$(printf '%s\n' "$matches" | wc -l)
fi
echo "$matches"

matches=$(grep -iF "FAIL:" "${RESULTS_FILE}" || true)

if [ -z "$matches" ]; then
    num_matches=0
else
    num_matches=$(printf '%s\n' "$matches" | wc -l)
fi
echo "$matches"
echo "${pass_num_matches} tests passing"
echo "${num_matches} tests failing"


matches=$(grep -iF ": Test_TestAndDocumentationRender" "${RESULTS_FILE}" || true)

if [ -z "$matches" ]; then
    total_num_matches=0
else
    total_num_matches=$(printf '%s\n' "$matches" | wc -l)
fi

echo "${total_num_matches} tests in total"
