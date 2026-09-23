#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
CANONICAL="$ROOT_DIR/SKILLS/hyperbricks"
PACKAGED="$ROOT_DIR/codex-plugin/hyperbricks/skills/hyperbricks"

if find "$PACKAGED" -name '.DS_Store' -print -quit | grep -q .; then
  echo "Packaged skill contains .DS_Store" >&2
  exit 1
fi

if ! diff -qr "$CANONICAL" "$PACKAGED"; then
  echo "Codex HyperBricks plugin skill is out of sync." >&2
  exit 1
fi

if rg -n 'Hyperbricks_logo' "$PACKAGED"; then
  echo "Packaged skill contains stale logo references" >&2
  exit 1
fi

echo "Codex HyperBricks plugin skill is synchronized."
