#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT_DIR"

command -v npm >/dev/null 2>&1 || { echo "npm is required" >&2; exit 1; }

echo "Installing JavaScript dependencies..."
npm ci

echo "Installing Playwright Chromium..."
npx playwright install chromium

echo "Building module plugins required by visual demos..."
scripts/plugins/build_hyperbricks_plugins.sh

echo "Running module screenshot checks..."
npm run test:module-screenshots
