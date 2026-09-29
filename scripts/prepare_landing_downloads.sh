#!/usr/bin/env bash
# Build release downloads, write SHA-256 sidecars, and copy them to a landing site.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PLUGIN_MANIFEST="${ROOT}/codex-plugin/hyperbricks/.codex-plugin/plugin.json"
VSCODE_DIR="${ROOT}/editors/vscode"

usage() {
    cat <<'USAGE'
Usage: ./scripts/prepare_landing_downloads.sh /absolute/path/to/static/downloads

Builds and verifies the committed Codex plugin and the checked-out VS Code
extension, creates SHA-256 sidecars, and copies these four untracked publication
artifacts to the specified landing-site downloads directory:

  hyperbricks-codex-plugin-<plugin-version>.zip
  hyperbricks-codex-plugin-<plugin-version>.zip.sha256
  hyperbricks-vscode-<extension-version>+vscode.<commit-UTC-timestamp>.vsix
  hyperbricks-vscode-<extension-version>+vscode.<commit-UTC-timestamp>.vsix.sha256

It also writes ../../resources/json/downloads.json relative to the downloads
directory, mapping the four stable artifact keys to these generated filenames.

The script does not edit landing-site templates, Git indexes, commits, or tags.
USAGE
}

if [[ $# -ne 1 || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    [[ $# -eq 1 ]] && exit 0
    exit 2
fi

DESTINATION="$1"
if [[ "${DESTINATION}" != /* || "${DESTINATION}" == "/" ]]; then
    echo "Destination must be an absolute downloads-directory path, not '/'." >&2
    exit 2
fi
if [[ ! -f "${PLUGIN_MANIFEST}" || ! -f "${VSCODE_DIR}/package.json" ]]; then
    echo "Missing Codex plugin manifest or VS Code extension checkout." >&2
    exit 1
fi

cd "${ROOT}"

if [[ -n "$(git -C "${VSCODE_DIR}" status --porcelain --untracked-files=all)" ]]; then
    echo "The VS Code extension checkout has uncommitted files; refusing to package it." >&2
    exit 1
fi

plugin_version="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["version"])' "${PLUGIN_MANIFEST}")"
vscode_version="$(node -p 'require(process.argv[1]).version' "${VSCODE_DIR}/package.json")"
vscode_commit_epoch="$(git -C "${VSCODE_DIR}" show -s --format=%ct HEAD)"
vscode_timestamp="$(python3 -c 'from datetime import datetime, timezone; import sys; print(datetime.fromtimestamp(int(sys.argv[1]), tz=timezone.utc).strftime("%Y%m%d%H%M%S"))' "${vscode_commit_epoch}")"
version_pattern='^[0-9A-Za-z][0-9A-Za-z.+_-]*$'
if [[ ! "${plugin_version}" =~ ${version_pattern} || ! "${vscode_version}" =~ ${version_pattern} || ! "${vscode_timestamp}" =~ ^[0-9]{14}$ ]]; then
    echo "Plugin or VS Code extension version is unsafe for an artifact filename." >&2
    exit 1
fi

STAGING="$(mktemp -d "${TMPDIR:-/tmp}/hyperbricks-downloads.XXXXXX")"
trap 'rm -rf "${STAGING}"' EXIT

plugin_name="hyperbricks-codex-plugin-${plugin_version}.zip"
vscode_name="hyperbricks-vscode-${vscode_version}+vscode.${vscode_timestamp}.vsix"
vscode_ignore="${STAGING}/vscodeignore"

cp "${VSCODE_DIR}/.vscodeignore" "${vscode_ignore}"
printf '\n.dev/**\n' >> "${vscode_ignore}"

python3 "${SCRIPT_DIR}/package_codex_plugin.py" \
    --output "${STAGING}/${plugin_name}"

(
    cd "${VSCODE_DIR}"
    npm run check
    ./node_modules/.bin/vsce package \
        --ignoreFile "${vscode_ignore}" \
        --out "${STAGING}/${vscode_name}"
)

python3 -c 'import sys, zipfile; names = zipfile.ZipFile(sys.argv[1]).namelist(); forbidden = [name for name in names if name.startswith("extension/.dev/")]; sys.exit(f"VSIX contains forbidden development runtime files: {forbidden}") if forbidden else None' "${STAGING}/${vscode_name}"

write_sha256() {
    local filename="$1"
    (
        cd "${STAGING}"
        if command -v sha256sum >/dev/null 2>&1; then
            sha256sum "${filename}" > "${filename}.sha256"
        else
            shasum -a 256 "${filename}" > "${filename}.sha256"
        fi
    )
}

write_sha256 "${plugin_name}"
write_sha256 "${vscode_name}"

downloads_json="${STAGING}/downloads.json"
python3 -c 'import json, pathlib, sys; target = pathlib.Path(sys.argv[1]); values = {"hyperbricks-codex-plugin": sys.argv[2], "hyperbricks-codex-plugin-sha": sys.argv[3], "hyperbricks-vscode": sys.argv[4], "hyperbricks-vscode-sha": sys.argv[5]}; target.write_text(json.dumps(values, indent=2) + "\n", encoding="utf-8")' \
    "${downloads_json}" \
    "${plugin_name}" \
    "${plugin_name}.sha256" \
    "${vscode_name}" \
    "${vscode_name}.sha256"

mkdir -p "${DESTINATION}"
cp "${STAGING}/${plugin_name}" "${DESTINATION}/${plugin_name}"
cp "${STAGING}/${plugin_name}.sha256" "${DESTINATION}/${plugin_name}.sha256"
cp "${STAGING}/${vscode_name}" "${DESTINATION}/${vscode_name}"
cp "${STAGING}/${vscode_name}.sha256" "${DESTINATION}/${vscode_name}.sha256"

JSON_DESTINATION="${DESTINATION}/../../resources/json"
mkdir -p "${JSON_DESTINATION}"
cp "${downloads_json}" "${JSON_DESTINATION}/downloads.json"

echo "Prepared landing-page downloads in ${DESTINATION}:"
echo "  ${plugin_name}"
echo "  ${plugin_name}.sha256"
echo "  ${vscode_name}"
echo "  ${vscode_name}.sha256"
echo "Wrote download filename map to ${JSON_DESTINATION}/downloads.json"
