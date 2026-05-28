#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
PORT="${HYPERBRICKS_PLUGIN_TEST_PORT:-18092}"
MODULE_NAME="${HYPERBRICKS_PLUGIN_TEST_MODULE:-hyperbricks-patterns-yaml}"
TMP_DIR="$(mktemp -d)"
SERVER_LOG="$TMP_DIR/server.log"
SERVER_PID=""

cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

cd "$ROOT_DIR"

echo "Starting HyperBricks plugin smoke runtime on port $PORT..."
env GOWORK=off HYPERBRICKS_LOCAL_PATH="$ROOT_DIR" \
  go run ./cmd/hyperbricks start -m "$MODULE_NAME" -p "$PORT" --non-interactive \
  >"$SERVER_LOG" 2>&1 &
SERVER_PID="$!"

for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:$PORT/fragments/status-demo-summary" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    echo "HyperBricks runtime exited while starting." >&2
    cat "$SERVER_LOG" >&2
    exit 1
  fi
  sleep 0.5
done

request() {
  local path="$1"
  local name="$2"
  local headers="$TMP_DIR/$name.headers"
  local body="$TMP_DIR/$name.body"

  curl -sS -D "$headers" "http://127.0.0.1:$PORT$path" -o "$body"
  if ! grep -qi '^X-Hyperbricks-Render-Error-Count: 0' "$headers"; then
    echo "Expected $path to render with X-Hyperbricks-Render-Error-Count: 0" >&2
    echo "--- headers ---" >&2
    cat "$headers" >&2
    echo "--- body ---" >&2
    cat "$body" >&2
    echo "--- server log ---" >&2
    tail -n 120 "$SERVER_LOG" >&2
    exit 1
  fi
}

assert_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq "$expected" "$file"; then
    echo "Expected output to contain: $expected" >&2
    echo "--- body ---" >&2
    cat "$file" >&2
    exit 1
  fi
}

request "/menu-demo" "menu-demo"
assert_contains "$TMP_DIR/menu-demo.body" "HTMX Enabled MENU"

request "/fragments/status-demo-plugin" "status-demo-plugin"
assert_contains "$TMP_DIR/status-demo-plugin.body" "Plugin panel"
assert_contains "$TMP_DIR/status-demo-plugin.body" "Template Config Plugin"

request "/docs/readme" "docs-readme"
assert_contains "$TMP_DIR/docs-readme.body" "HyperBricks Patterns"
assert_contains "$TMP_DIR/docs-readme.body" 'hx-select-oob="#pattern-docs-sidebar-shell:outerHTML"'
if grep -Fq 'hx-select="#pattern-docs-panel > *, #pattern-docs-sidebar-shell"' "$TMP_DIR/docs-readme.body"; then
  echo "Docs navigation must update the sidebar out-of-band, not inside the panel selection." >&2
  exit 1
fi
if [[ "$(grep -Fo 'id="pattern-docs-sidebar-shell"' "$TMP_DIR/docs-readme.body" | wc -l | tr -d ' ')" != "1" ]]; then
  echo "Expected docs sidebar shell id to appear exactly once." >&2
  exit 1
fi

request "/guarded-demo" "guarded-demo"
assert_contains "$TMP_DIR/guarded-demo.body" "Login page"

guard_headers="$TMP_DIR/guarded-secret-redirect.headers"
guard_body="$TMP_DIR/guarded-secret-redirect.body"
curl -sS -D "$guard_headers" "http://127.0.0.1:$PORT/guarded-demo/secret" -o "$guard_body"
if ! grep -qi '^HTTP/1.1 303' "$guard_headers" || ! grep -qi '^Location: /guarded-demo/login' "$guard_headers"; then
  echo "Expected unauthenticated guarded route to redirect to /guarded-demo/login" >&2
  echo "--- headers ---" >&2
  cat "$guard_headers" >&2
  echo "--- body ---" >&2
  cat "$guard_body" >&2
  exit 1
fi

login_headers="$TMP_DIR/guarded-login-hx.headers"
login_body="$TMP_DIR/guarded-login-hx.body"
curl -sS -D "$login_headers" \
  -H "HX-Request: true" \
  -H "HX-Current-URL: http://127.0.0.1:$PORT/guarded-demo" \
  -X POST \
  -d "identifier=demo&password=open-sesame" \
  "http://127.0.0.1:$PORT/guarded-demo/auth/login" \
  -o "$login_body"
if ! grep -qi '^HX-Redirect: /guarded-demo/secret' "$login_headers"; then
  echo "Expected HTMX login to return HX-Redirect: /guarded-demo/secret" >&2
  echo "--- headers ---" >&2
  cat "$login_headers" >&2
  echo "--- body ---" >&2
  cat "$login_body" >&2
  exit 1
fi

secret_headers="$TMP_DIR/guarded-secret.headers"
secret_body="$TMP_DIR/guarded-secret.body"
curl -sS -D "$secret_headers" \
  -H "Cookie: guarded_demo_session=allow" \
  "http://127.0.0.1:$PORT/guarded-demo/secret" \
  -o "$secret_body"
if ! grep -qi '^X-Hyperbricks-Render-Error-Count: 0' "$secret_headers"; then
  echo "Expected authenticated guarded route to render with X-Hyperbricks-Render-Error-Count: 0" >&2
  echo "--- headers ---" >&2
  cat "$secret_headers" >&2
  echo "--- body ---" >&2
  cat "$secret_body" >&2
  exit 1
fi
assert_contains "$secret_body" "Access granted"

echo "Plugin smoke tests passed."
