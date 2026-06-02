#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
MODULE_NAME="hyperbricks-patterns-yaml"
HYPERBRICKS_LOCAL_PATH="$ROOT_DIR"
DRY_RUN=0
BUILD_CORE=1
BUILD_CUSTOM=1

CORE_PLUGINS=(
  "esbuild@2.0.0"
  "tailwindcss@2.0.0"
  "markdown@2.0.0"
)

usage() {
  cat <<'EOF'
Usage: scripts/plugins/build_hyperbricks_plugins.sh [options]

Builds HyperBricks plugins against this checkout:
  - shared core plugins from ./plugins/<name>/<version>
  - custom module plugins from modules/<module>/plugins/<name>/<version>

Options:
  --module <name>            Module whose custom plugins should be built.
                             Default: hyperbricks-patterns-yaml
  --hyperbricks-path <path>  Local HyperBricks checkout used in plugin go.mod
                             replace directives. Default: this repository root.
  --skip-core                Do not build shared core plugins.
  --skip-custom              Do not build module custom plugins.
  --dry-run                  Print build commands without running them.
  -h, --help                 Show this help text.

Notes:
  This test wrapper intentionally pins HYPERBRICKS_LOCAL_PATH to this checkout
  unless --hyperbricks-path is passed. Inherited shell values can make Go plugin
  build IDs differ from the runtime that opens them.

  GOWORK is ignored by this script. Plugin builds are forced to GOWORK=off to
  avoid workspace graph mismatches.

Examples:
  scripts/plugins/build_hyperbricks_plugins.sh
  scripts/plugins/build_hyperbricks_plugins.sh --dry-run
  scripts/plugins/build_hyperbricks_plugins.sh --module hyperbricks-patterns-yaml
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --module)
      MODULE_NAME="${2:-}"
      if [[ -z "$MODULE_NAME" ]]; then
        echo "--module requires a value" >&2
        exit 2
      fi
      shift 2
      ;;
    --hyperbricks-path)
      HYPERBRICKS_LOCAL_PATH="${2:-}"
      if [[ -z "$HYPERBRICKS_LOCAL_PATH" ]]; then
        echo "--hyperbricks-path requires a value" >&2
        exit 2
      fi
      shift 2
      ;;
    --skip-core)
      BUILD_CORE=0
      shift
      ;;
    --skip-custom)
      BUILD_CUSTOM=0
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! HYPERBRICKS_LOCAL_PATH="$(cd "$HYPERBRICKS_LOCAL_PATH" 2>/dev/null && pwd -P)"; then
  echo "HYPERBRICKS_LOCAL_PATH must point to an existing local HyperBricks checkout" >&2
  echo "Configured path: ${HYPERBRICKS_LOCAL_PATH:-<empty>}" >&2
  exit 1
fi

if [[ -z "$HYPERBRICKS_LOCAL_PATH" || ! -f "$HYPERBRICKS_LOCAL_PATH/go.mod" ]]; then
  echo "HYPERBRICKS_LOCAL_PATH must point to a local HyperBricks checkout with go.mod" >&2
  echo "Resolved path: ${HYPERBRICKS_LOCAL_PATH:-<empty>}" >&2
  exit 1
fi

cd "$ROOT_DIR"

BIN_PLUGIN_DIR="$ROOT_DIR/bin/plugins"
MODULE_PLUGIN_DIR="$ROOT_DIR/modules/$MODULE_NAME/plugins"
BUILD_MARKER="$BIN_PLUGIN_DIR/hyperbricks-plugin-build-ok"

if [[ "$DRY_RUN" -eq 0 ]]; then
  rm -f "$BUILD_MARKER"
fi

run_cmd() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf 'DRY RUN:'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

run_hyperbricks() {
  run_cmd env \
    GOWORK=off \
    HYPERBRICKS_LOCAL_PATH="$HYPERBRICKS_LOCAL_PATH" \
    go run ./cmd/hyperbricks "$@"
}

manifest_value() {
  local manifest="$1"
  local key="$2"
  sed -nE "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"([^\"]+)\".*/\\1/p" "$manifest" | head -n 1
}

camel_case_go_file() {
  local file="$1"
  local base="${file%.go}"
  local part
  local out=""

  IFS='_' read -r -a parts <<< "$base"
  for part in "${parts[@]}"; do
    if [[ -z "$part" ]]; then
      continue
    fi
    local first
    first="$(printf '%s' "${part:0:1}" | tr '[:lower:]' '[:upper:]')"
    out+="${first}${part:1}"
  done
  printf '%s' "$out"
}

plugin_output_name() {
  local plugin="$1"
  local module="${2:-}"
  local name="${plugin%@*}"
  local version="${plugin##*@}"
  local source_dir

  if [[ -n "$module" ]]; then
    source_dir="$ROOT_DIR/modules/$module/plugins/$name/$version"
  else
    source_dir="$ROOT_DIR/plugins/$name/$version"
  fi

  local manifest="$source_dir/manifest.json"
  if [[ ! -f "$manifest" ]]; then
    echo "manifest.json not found at $manifest" >&2
    return 1
  fi

  local binary
  binary="$(manifest_value "$manifest" "binary")"
  if [[ -z "$binary" ]]; then
    local source
    source="$(manifest_value "$manifest" "source")"
    if [[ -z "$source" ]]; then
      echo "source field is missing in $manifest" >&2
      return 1
    fi
    binary="$(camel_case_go_file "$source")"
  fi
  binary="${binary%.so}"

  if [[ -n "$module" ]]; then
    binary="${binary}__${module}"
  fi

  printf '%s@%s.so' "$binary" "$version"
}

plugin_source_dir() {
  local plugin="$1"
  local module="${2:-}"
  local name="${plugin%@*}"
  local version="${plugin##*@}"

  if [[ -n "$module" ]]; then
    printf '%s/modules/%s/plugins/%s/%s' "$ROOT_DIR" "$module" "$name" "$version"
  else
    printf '%s/plugins/%s/%s' "$ROOT_DIR" "$name" "$version"
  fi
}

snapshot_go_mod_files() {
  local source_dir="$1"
  local snapshot_dir="$2"

  if [[ -f "$source_dir/go.mod" ]]; then
    cp "$source_dir/go.mod" "$snapshot_dir/go.mod"
  fi
  if [[ -f "$source_dir/go.sum" ]]; then
    cp "$source_dir/go.sum" "$snapshot_dir/go.sum"
  fi
}

restore_go_mod_files() {
  local source_dir="$1"
  local snapshot_dir="$2"

  if [[ -f "$snapshot_dir/go.mod" ]]; then
    cp "$snapshot_dir/go.mod" "$source_dir/go.mod"
  fi
  if [[ -f "$snapshot_dir/go.sum" ]]; then
    cp "$snapshot_dir/go.sum" "$source_dir/go.sum"
  else
    rm -f "$source_dir/go.sum"
  fi
}

build_plugin() {
  local plugin="$1"
  local module="${2:-}"
  local source_dir
  local output_name
  local snapshot_dir
  local status=0

  source_dir="$(plugin_source_dir "$plugin" "$module")"
  output_name="$(plugin_output_name "$plugin" "$module")"

  if [[ ! -d "$source_dir" ]]; then
    echo "Plugin source directory not found: $source_dir" >&2
    return 1
  fi

  if [[ "$DRY_RUN" -eq 1 ]]; then
    if [[ -n "$module" ]]; then
      run_hyperbricks plugin build "$plugin" --module "$module"
    else
      run_hyperbricks plugin build "$plugin"
    fi
    return 0
  fi

  snapshot_dir="$(mktemp -d)"
  snapshot_go_mod_files "$source_dir" "$snapshot_dir"
  rm -f "$BIN_PLUGIN_DIR/$output_name"

  set +e
  if [[ -n "$module" ]]; then
    run_hyperbricks plugin build "$plugin" --module "$module"
  else
    run_hyperbricks plugin build "$plugin"
  fi
  status=$?
  set -e

  restore_go_mod_files "$source_dir" "$snapshot_dir"
  rm -rf "$snapshot_dir"

  if [[ "$status" -ne 0 ]]; then
    return "$status"
  fi
  if [[ ! -f "$BIN_PLUGIN_DIR/$output_name" ]]; then
    echo "Expected plugin binary was not created: $BIN_PLUGIN_DIR/$output_name" >&2
    return 1
  fi
}

CUSTOM_PLUGINS=()
if [[ "$BUILD_CUSTOM" -eq 1 ]]; then
  if [[ ! -d "$MODULE_PLUGIN_DIR" ]]; then
    echo "Module plugin directory not found: $MODULE_PLUGIN_DIR" >&2
    exit 1
  fi
  while IFS= read -r dir; do
    name="$(basename "$(dirname "$dir")")"
    version="$(basename "$dir")"
    CUSTOM_PLUGINS+=("${name}@${version}")
  done < <(find "$MODULE_PLUGIN_DIR" -mindepth 2 -maxdepth 2 -type d | sort)
fi

echo "Using HYPERBRICKS_LOCAL_PATH=$HYPERBRICKS_LOCAL_PATH"
if [[ "$BUILD_CORE" -eq 1 ]]; then
  echo "Building shared plugins:"
  for plugin in "${CORE_PLUGINS[@]}"; do
    echo "  - $plugin"
  done
fi
if [[ "$BUILD_CUSTOM" -eq 1 ]]; then
  echo "Building custom module plugins for $MODULE_NAME:"
  for plugin in "${CUSTOM_PLUGINS[@]}"; do
    echo "  - $plugin"
  done
fi

if [[ "$BUILD_CORE" -eq 1 ]]; then
  for plugin in "${CORE_PLUGINS[@]}"; do
    build_plugin "$plugin"
  done
fi

if [[ "$BUILD_CUSTOM" -eq 1 ]]; then
  for plugin in "${CUSTOM_PLUGINS[@]}"; do
    build_plugin "$plugin" "$MODULE_NAME"
  done
fi

if [[ "$DRY_RUN" -eq 0 ]]; then
  mkdir -p "$BIN_PLUGIN_DIR"
  {
    printf 'module=%s\n' "$MODULE_NAME"
    printf 'core=%s\n' "$([[ "$BUILD_CORE" -eq 1 ]] && echo true || echo false)"
    printf 'custom=%s\n' "$([[ "$BUILD_CUSTOM" -eq 1 ]] && echo true || echo false)"
    printf 'hyperbricks_local_path=%s\n' "$HYPERBRICKS_LOCAL_PATH"
    date -u '+built_at=%Y-%m-%dT%H:%M:%SZ'
  } > "$BUILD_MARKER"
fi

echo "Plugin build script completed."
