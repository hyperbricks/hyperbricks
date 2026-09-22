#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ENV_DIR="${HANDBOOK_VENV:-${ROOT}/.venv-handbooks}"
REQUIREMENTS="${SCRIPT_DIR}/handbook-requirements.txt"
MERMAID_VERSION="11.17.0"
MERMAID_DIR="${ENV_DIR}/mermaid"

if [[ ! -x "${ENV_DIR}/bin/python" ]]; then
    "${PYTHON:-python3}" -m venv "${ENV_DIR}"
fi

dependencies_healthy() {
    "${ENV_DIR}/bin/python" - "${REQUIREMENTS}" <<'PY'
from importlib.metadata import PackageNotFoundError, version
from pathlib import Path
import sys

for line in Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    requirement = line.strip()
    if not requirement or requirement.startswith("#"):
        continue
    name, expected = requirement.split("==", 1)
    try:
        actual = version(name)
    except PackageNotFoundError:
        sys.exit(1)
    if actual != expected:
        sys.exit(1)
PY
}

if ! cmp -s "${REQUIREMENTS}" "${ENV_DIR}/handbook-requirements.txt" || ! dependencies_healthy; then
    if dependencies_healthy; then
        "${ENV_DIR}/bin/python" -m pip install -r "${REQUIREMENTS}"
    else
        # A partial install may have a dist-info directory without METADATA;
        # ordinary pip installation cannot inspect or replace that state.
        "${ENV_DIR}/bin/python" -m pip install --ignore-installed -r "${REQUIREMENTS}"
    fi
    dependencies_healthy
    cp "${REQUIREMENTS}" "${ENV_DIR}/handbook-requirements.txt"
fi

if [[ ! -x "${MERMAID_DIR}/node_modules/.bin/mmdc" ]] || [[ "$(cat "${MERMAID_DIR}/version" 2>/dev/null || true)" != "${MERMAID_VERSION}" ]]; then
    npm install --prefix "${MERMAID_DIR}" --no-save --package-lock=false "@mermaid-js/mermaid-cli@${MERMAID_VERSION}"
    printf '%s\n' "${MERMAID_VERSION}" > "${MERMAID_DIR}/version"
fi
export HB_MERMAID_CLI="${MERMAID_DIR}/node_modules/.bin/mmdc"

exec "${ENV_DIR}/bin/python" "${SCRIPT_DIR}/build_handbooks.py" "$@"
