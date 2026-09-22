#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
CANONICAL="$ROOT_DIR/SKILLS/hyperbricks"
PACKAGED="$ROOT_DIR/codex-plugin/hyperbricks/skills/hyperbricks"

files=(
  SKILL.md
  agents/openai.yaml
  references/authoring.md
  references/integrations.md
  references/plugins.md
  references/project-lifecycle.md
  references/scaffolding.md
  references/spaces-markdown.md
  references/HyperBricks-Skills.md
  references/HyperBricks-Documentation.md
  assets/hyperbricks_round_black_on_white.svg
  assets/hyperbricks_round_white_on_black.svg
)

for file in "${files[@]}"; do
  if [[ ! -f "$CANONICAL/$file" || ! -f "$PACKAGED/$file" ]]; then
    echo "Missing synchronized file: $file" >&2
    exit 1
  fi
  if ! cmp -s "$CANONICAL/$file" "$PACKAGED/$file"; then
    echo "Out of sync: $file" >&2
    exit 1
  fi
done

if find "$PACKAGED" -name '.DS_Store' -print -quit | grep -q .; then
  echo "Packaged skill contains .DS_Store" >&2
  exit 1
fi

if rg -n 'Hyperbricks_logo' "$PACKAGED"; then
  echo "Packaged skill contains stale logo references" >&2
  exit 1
fi

echo "Codex HyperBricks plugin skill is synchronized."
