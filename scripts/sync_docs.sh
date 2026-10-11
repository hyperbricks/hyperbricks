#!/usr/bin/env bash
# Synchronize publication compilations and the shared skill snapshot from docs/.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CANONICAL="${ROOT}/.agents/skills/hyperbricks"

for argument in "$@"; do
    case "${argument}" in
        -h|--help)
            echo "Usage: ./scripts/sync_docs.sh"
            echo "Build documentation compilations and synchronize the shared skill from docs/."
            exit 0 ;;
        *) echo "Unknown option: ${argument}" >&2; exit 2 ;;
    esac
done

cd "${ROOT}"
if [[ ! -f "${CANONICAL}/SKILL.md" ]]; then
    echo "Missing canonical skill; refusing to synchronize." >&2
    exit 1
fi
if [[ -L "${CANONICAL}" || -L "${ROOT}/.agents" || -L "${ROOT}/.agents/skills" ]]; then
    echo "Symlinked canonical skill directory; refusing to synchronize." >&2
    exit 1
fi

bash "${SCRIPT_DIR}/compilation-generation/build_compilations.sh" --ref WORKTREE

PYTHON="${COMPILATION_VENV:-${ROOT}/.venv-compilations}/bin/python"
"${PYTHON}" -m unittest discover -s scripts/compilation-generation -p 'test_build*.py'
python3 -m unittest scripts/test_package_agent_skill.py scripts/test_agent_pipeline.py
go test ./test/docs
"${PYTHON}" scripts/compilation-generation/build_markdown_compilations.py --check --ref WORKTREE
"${PYTHON}" scripts/compilation-generation/build_skill_documentation.py --check
git --no-pager diff --check

echo "Documentation compilations and shared skill references are synchronized."
echo "No files were committed, tagged, or pushed. Package downloads after committing the reviewed sources."
