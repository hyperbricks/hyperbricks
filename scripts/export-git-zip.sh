#!/usr/bin/env bash

set -euo pipefail

if (( $# > 1 )); then
  printf 'Usage: %s [output-directory]\n' "$(basename "$0")" >&2
  exit 2
fi

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
repo_root=$(git -C "$script_dir/.." rev-parse --show-toplevel)
revision=HEAD
short_sha=$(git -C "$repo_root" rev-parse --short "$revision")
version=$(git -C "$repo_root" show "$revision:assets/version.md" | tr -d '[:space:]')

if [[ -z "$version" || "$version" == *[!A-Za-z0-9._-]* ]]; then
  printf 'Invalid version in committed assets/version.md: %q\n' "$version" >&2
  exit 1
fi

if (( $# == 1 )); then
  output_dir=$1
else
  output_dir=$(dirname "$repo_root")
fi

if [[ ! -d "$output_dir" ]]; then
  printf 'Output directory does not exist: %s\n' "$output_dir" >&2
  exit 1
fi
output_dir=$(cd "$output_dir" && pwd -P)

archive_name="hyperbricks-${version}-${short_sha}.zip"
archive_path="$output_dir/$archive_name"

if [[ -n "$(git -C "$repo_root" status --porcelain)" ]]; then
  printf 'Note: the archive contains committed HEAD only; working-tree changes are excluded.\n' >&2
fi

git -C "$repo_root" archive \
  --format=zip \
  --prefix=hyperbricks/ \
  --output="$archive_path" \
  "$revision"

printf 'HyperBricks source archive created: %s\n' "$archive_path"
printf 'Archived version %s at commit %s.\n' "$version" "$short_sha"
