#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ENV_DIR="${COMPILATION_VENV:-${ROOT}/.venv-compilations}"
REQUIREMENTS="${SCRIPT_DIR}/compilation-requirements.txt"
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

if ! cmp -s "${REQUIREMENTS}" "${ENV_DIR}/compilation-requirements.txt" || ! dependencies_healthy; then
    if dependencies_healthy; then
        "${ENV_DIR}/bin/python" -m pip install -r "${REQUIREMENTS}"
    else
        # A partial install may have a dist-info directory without METADATA;
        # ordinary pip installation cannot inspect or replace that state.
        "${ENV_DIR}/bin/python" -m pip install --ignore-installed -r "${REQUIREMENTS}"
    fi
    dependencies_healthy
    cp "${REQUIREMENTS}" "${ENV_DIR}/compilation-requirements.txt"
fi

if [[ ! -x "${MERMAID_DIR}/node_modules/.bin/mmdc" ]] || [[ "$(cat "${MERMAID_DIR}/version" 2>/dev/null || true)" != "${MERMAID_VERSION}" ]]; then
    npm install --prefix "${MERMAID_DIR}" --no-save --package-lock=false "@mermaid-js/mermaid-cli@${MERMAID_VERSION}"
    printf '%s\n' "${MERMAID_VERSION}" > "${MERMAID_DIR}/version"
fi
export HB_MERMAID_CLI="${MERMAID_DIR}/node_modules/.bin/mmdc"

FORMAT="all"
OUTPUT_DIR="${ROOT}/docs/compilations"
SOURCE_REF="WORKTREE"
BUILD_ARGUMENT_COUNT=$#
BUILD_ARGUMENTS=("$@")
while (($#)); do
    case "$1" in
        --format)
            if (($# > 1)); then FORMAT="$2"; shift 2; else shift; fi
            ;;
        --format=*)
            FORMAT="${1#--format=}"
            shift
            ;;
        --output-dir)
            if (($# > 1)); then OUTPUT_DIR="$2"; shift 2; else shift; fi
            ;;
        --output-dir=*)
            OUTPUT_DIR="${1#--output-dir=}"
            shift
            ;;
        --ref)
            if (($# > 1)); then SOURCE_REF="$2"; shift 2; else shift; fi
            ;;
        --ref=*)
            SOURCE_REF="${1#--ref=}"
            shift
            ;;
        *)
            shift
            ;;
    esac
done

if [[ "${OUTPUT_DIR}" != /* ]]; then
    OUTPUT_DIR="${ROOT}/${OUTPUT_DIR}"
fi

if ((BUILD_ARGUMENT_COUNT)); then
    "${ENV_DIR}/bin/python" "${SCRIPT_DIR}/build_compilations.py" "${BUILD_ARGUMENTS[@]}"
else
    "${ENV_DIR}/bin/python" "${SCRIPT_DIR}/build_compilations.py"
fi

if [[ "${FORMAT}" == "all" || "${FORMAT}" == "markdown" ]]; then
    COMPILATIONS=(
        HyperBricks-Documentation.md
    )
    DESTINATIONS=(
        "${ROOT}/SKILLS/hyperbricks/references"
        "${ROOT}/codex-plugin/hyperbricks/skills/hyperbricks/references"
    )

    for compilation in "${COMPILATIONS[@]}"; do
        if [[ ! -f "${OUTPUT_DIR}/${compilation}" ]]; then
            echo "Generated compilation is missing: ${OUTPUT_DIR}/${compilation}" >&2
            exit 1
        fi
    done

    for destination in "${DESTINATIONS[@]}"; do
        for compilation in "${COMPILATIONS[@]}"; do
            cp "${OUTPUT_DIR}/${compilation}" "${destination}/${compilation}"
            echo "Synchronized ${destination#"${ROOT}/"}/${compilation}"
        done
        "${ENV_DIR}/bin/python" "${SCRIPT_DIR}/build_skill_documentation.py" \
            --ref "${SOURCE_REF}" \
            --output-dir "${destination}"
    done
fi
