#!/usr/bin/env bash
# Build generated repository documentation, run tests, then synchronize release documentation.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

usage() {
    cat <<'USAGE'
Usage: ./scripts/pre_release.sh

Runs the ordered pre-release pipeline:
  1. ./tests.sh --with-docs --with-plugins --with-modules
                --with-screenshots
  2. ./scripts/sync_docs.sh

All documentation, plugin, module, and screenshot checks are enabled
automatically. This script does not accept test-scope options.
USAGE
}

for argument in "$@"; do
    case "${argument}" in
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: ${argument}" >&2
            usage >&2
            exit 2
            ;;
    esac
done

cd "${ROOT}"
bash "${ROOT}/tests.sh" \
    --with-docs \
    --with-plugins \
    --with-modules \
    --with-screenshots
bash "${SCRIPT_DIR}/sync_docs.sh"

echo "Pre-release documentation and test checks passed."
