#!/usr/bin/env bash
# Build release downloads, write SHA-256 sidecars, and copy them to a landing site.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
VERSION_FILE="${ROOT}/assets/version.md"
VSCODE_DIR="${ROOT}/editors/vscode"

usage() {
    cat <<'USAGE'
Usage: ./scripts/prepare_landing_downloads.sh /absolute/path/to/static/downloads

Builds and verifies the committed shared HyperBricks skill and checked-out
VS Code extension, creates SHA-256 sidecars, and copies publication
artifacts to the specified landing-site downloads directory:

  hyperbricks-skill-<runtime-version>.zip
  hyperbricks-skill-<runtime-version>.zip.sha256
  hyperbricks-vscode-<extension-version>+vscode.<commit-UTC-timestamp>.vsix
  hyperbricks-vscode-<extension-version>+vscode.<commit-UTC-timestamp>.vsix.sha256

It also writes ../../resources/json/downloads.json relative to the downloads
directory, mapping the selected artifact keys to these generated filenames.

The script does not edit landing-site templates, Git indexes, commits, or tags.
USAGE
}

DESTINATION=""
for argument in "$@"; do
    case "${argument}" in
        -h|--help) usage; exit 0 ;;
        --*) echo "Unknown option: ${argument}" >&2; exit 2 ;;
        *)
            if [[ -n "${DESTINATION}" ]]; then usage >&2; exit 2; fi
            DESTINATION="${argument}"
            ;;
    esac
done
if [[ -z "${DESTINATION}" ]]; then usage >&2; exit 2; fi

if [[ "${DESTINATION}" != /* || "${DESTINATION}" == "/" ]]; then
    echo "Destination must be an absolute downloads-directory path, not '/'." >&2
    exit 2
fi
if [[ ! -f "${VSCODE_DIR}/package.json" ]]; then
    echo "Missing VS Code extension checkout." >&2
    exit 1
fi

cd "${ROOT}"

if [[ -n "$(git -C "${VSCODE_DIR}" status --porcelain --untracked-files=all)" ]]; then
    echo "The VS Code extension checkout has uncommitted files; refusing to package it." >&2
    exit 1
fi
recorded_vscode_commit="$(git rev-parse HEAD:editors/vscode)"
if [[ "$(git -C "${VSCODE_DIR}" rev-parse HEAD)" != "${recorded_vscode_commit}" ]]; then
    echo "The VS Code extension HEAD differs from the committed submodule revision." >&2
    exit 1
fi

skill_version="$(cat "${VERSION_FILE}")"
vscode_version="$(node -p 'require(process.argv[1]).version' "${VSCODE_DIR}/package.json")"
vscode_commit_epoch="$(git -C "${VSCODE_DIR}" show -s --format=%ct HEAD)"
vscode_timestamp="$(python3 -c 'from datetime import datetime, timezone; import sys; print(datetime.fromtimestamp(int(sys.argv[1]), tz=timezone.utc).strftime("%Y%m%d%H%M%S"))' "${vscode_commit_epoch}")"
version_pattern='^[0-9A-Za-z][0-9A-Za-z.+_-]*$'
if [[ ! "${vscode_version}" =~ ${version_pattern} || ! "${vscode_timestamp}" =~ ^[0-9]{14}$ ||
      ! "${skill_version}" =~ ${version_pattern} ]]; then
    echo "HyperBricks or VS Code extension version is unsafe for an artifact filename." >&2
    exit 1
fi

STAGING="$(mktemp -d "${TMPDIR:-/tmp}/hyperbricks-downloads.XXXXXX")"
trap 'rm -rf "${STAGING}"' EXIT

skill_name="hyperbricks-skill-${skill_version}.zip"
vscode_name="hyperbricks-vscode-${vscode_version}+vscode.${vscode_timestamp}.vsix"
vscode_ignore="${STAGING}/vscodeignore"

cp "${VSCODE_DIR}/.vscodeignore" "${vscode_ignore}"
printf '\n.dev/**\n' >> "${vscode_ignore}"

python3 "${SCRIPT_DIR}/package_agent_skill.py" \
    --output "${STAGING}/${skill_name}"

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

write_sha256 "${vscode_name}"

verify_sha256() {
    python3 - "$1" "${skill_name}" "${vscode_name}" <<'PYCHECK'
import hashlib
from pathlib import Path
import sys
destination = Path(sys.argv[1])
for name in sys.argv[2:]:
    checksum, filename = (destination / (name + '.sha256')).read_text().split()
    if filename != name or hashlib.sha256((destination / name).read_bytes()).hexdigest() != checksum:
        sys.exit(f'Download checksum mismatch: {name}')
PYCHECK
}
verify_sha256 "${STAGING}"

downloads_json="${STAGING}/downloads.json"
python3 - "${downloads_json}" "${vscode_name}" "${skill_name}" <<'PYJSON'
import json
from pathlib import Path
import sys
values = {"hyperbricks-vscode": sys.argv[2], "hyperbricks-vscode-sha": sys.argv[2] + ".sha256"}
values.update({"hyperbricks-skill": sys.argv[3], "hyperbricks-skill-sha": sys.argv[3] + ".sha256"})
Path(sys.argv[1]).write_text(json.dumps(values, indent=2) + "\n", encoding="utf-8")
PYJSON

mkdir -p "${DESTINATION}"
cp "${STAGING}/${skill_name}" "${DESTINATION}/${skill_name}"
cp "${STAGING}/${skill_name}.sha256" "${DESTINATION}/${skill_name}.sha256"
cp "${STAGING}/${vscode_name}" "${DESTINATION}/${vscode_name}"
cp "${STAGING}/${vscode_name}.sha256" "${DESTINATION}/${vscode_name}.sha256"

verify_sha256 "${DESTINATION}"

JSON_DESTINATION="${DESTINATION}/../../resources/json"
mkdir -p "${JSON_DESTINATION}"
cp "${downloads_json}" "${JSON_DESTINATION}/downloads.json"

echo "Prepared landing-page downloads in ${DESTINATION}:"
echo "  ${skill_name}"
echo "  ${skill_name}.sha256"
echo "  ${vscode_name}"
echo "  ${vscode_name}.sha256"
echo "Wrote download filename map to ${JSON_DESTINATION}/downloads.json"
