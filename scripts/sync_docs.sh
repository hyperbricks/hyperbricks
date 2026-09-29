#!/usr/bin/env bash
# Synchronize documentation compilations and skill mirrors, then verify them.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CANONICAL="${ROOT}/SKILLS/hyperbricks"
PACKAGED="${ROOT}/codex-plugin/hyperbricks/skills/hyperbricks"

cd "${ROOT}"
if [[ ! -f "${CANONICAL}/SKILL.md" || ! -f "${ROOT}/codex-plugin/hyperbricks/.codex-plugin/plugin.json" ]]; then
    echo "Missing canonical skill or Codex plugin; refusing to synchronize." >&2
    exit 1
fi
if [[ -L "${CANONICAL}" || -L "${PACKAGED}" || -L "${ROOT}/codex-plugin/hyperbricks/skills" ]]; then
    echo "Symlinked skill directory; refusing to synchronize." >&2
    exit 1
fi
if ! command -v rsync >/dev/null; then
    echo "rsync is required to mirror the canonical skill tree." >&2
    exit 1
fi
if find "${CANONICAL}" \( -name '.DS_Store' -o -type l \) -print -quit | grep -q .; then
    echo "Canonical skill contains macOS metadata or symlinks; refusing to package it." >&2
    exit 1
fi

PACKAGED_BEFORE="$(mktemp -d "${TMPDIR:-/tmp}/hyperbricks-plugin-before.XXXXXX")"
trap 'rm -rf "${PACKAGED_BEFORE}"' EXIT
if [[ -d "${PACKAGED}" ]]; then
    cp -R "${PACKAGED}/." "${PACKAGED_BEFORE}/"
fi

bash "${SCRIPT_DIR}/compilation-generation/build_compilations.sh" --ref WORKTREE

# The packaged skill is an exact mirror, including the generated references.
mkdir -p "${PACKAGED}"
rsync -a --delete "${CANONICAL}/" "${PACKAGED}/"

# A changed mirror changes the distributed Codex plugin. Compare with the
# pre-generation snapshot because the compilation builder writes generated
# references directly into both the canonical and packaged skill trees.
python3 "${SCRIPT_DIR}/update_codex_plugin_version.py" \
    --manifest "${ROOT}/codex-plugin/hyperbricks/.codex-plugin/plugin.json" \
    --before-skill "${PACKAGED_BEFORE}" \
    --after-skill "${PACKAGED}"

PYTHON="${COMPILATION_VENV:-${ROOT}/.venv-compilations}/bin/python"
python3 -m unittest "${SCRIPT_DIR}/test_update_codex_plugin_version.py"
"${PYTHON}" -m unittest discover -s scripts/compilation-generation -p 'test_build*.py'
go test ./test/docs
"${PYTHON}" scripts/compilation-generation/build_markdown_compilations.py --check --ref WORKTREE
"${PYTHON}" scripts/compilation-generation/build_skill_documentation.py --check
"${PYTHON}" scripts/compilation-generation/build_skill_documentation.py --check \
    --output-dir codex-plugin/hyperbricks/skills/hyperbricks/references
bash "${SCRIPT_DIR}/check_codex_plugin_sync.sh"
git --no-pager diff --check

echo "Documentation compilations, skill references, and Codex plugin mirror are synchronized."
echo "No files were committed, tagged, or pushed."
