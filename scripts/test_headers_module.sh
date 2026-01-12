#!/bin/bash
set -euo pipefail
set +m

MODULE_DEV="headers-test"
MODULE_LIVE="headers-test-live"
PORT_DEV="8092"
PORT_LIVE="8093"
LOG_DIR="/tmp"
GOCACHE_DIR="${GOCACHE:-$(pwd)/.cache/go-build}"
export GOCACHE="${GOCACHE_DIR}"
mkdir -p "${GOCACHE_DIR}"
HB_CMD="${HB_CMD:-go run ./cmd/hyperbricks}"

HB_PID=""
HB_LISTEN_PID=""
HB_PORT=""

ensure_module_dirs() {
  local module="$1"
  local module_dir="modules/${module}"

  mkdir -p "${module_dir}/static" \
    "${module_dir}/resources" \
    "${module_dir}/templates" \
    "${module_dir}/rendered" \
    "./bin/plugins"
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

ensure_port_available() {
  local port="$1"

  if ! command_exists lsof; then
    return 0
  fi

  local listeners
  listeners=$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $1 ":" $2}' || true)
  if [[ -z "${listeners}" ]]; then
    return 0
  fi

  local blocked=""
  for entry in ${listeners}; do
    local cmd="${entry%%:*}"
    local pid="${entry##*:}"
    if [[ "${cmd}" == hyperbric* ]]; then
      echo "Port ${port} in use by ${cmd} (pid ${pid}); stopping it..."
      kill "${pid}" >/dev/null 2>&1 || true
      for _ in $(seq 1 10); do
        if ! kill -0 "${pid}" >/dev/null 2>&1; then
          break
        fi
        sleep 0.2
      done
      if kill -0 "${pid}" >/dev/null 2>&1; then
        kill -9 "${pid}" >/dev/null 2>&1 || true
      fi
    else
      echo "Port ${port} in use by ${cmd} (pid ${pid}). Stop it and re-run."
      blocked="yes"
    fi
  done

  if [[ -n "${blocked}" ]]; then
    exit 1
  fi

  if lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "Port ${port} still in use. Stop the process and re-run."
    exit 1
  fi
}

stop_listener_port() {
  local port="$1"

  if [[ -z "${port}" ]]; then
    return
  fi
  if ! command_exists lsof; then
    return
  fi

  local listeners
  listeners=$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $1 ":" $2}' || true)
  if [[ -z "${listeners}" ]]; then
    return
  fi

  for entry in ${listeners}; do
    local cmd="${entry%%:*}"
    local pid="${entry##*:}"
    if [[ "${cmd}" == hyperbric* ]]; then
      kill "${pid}" >/dev/null 2>&1 || true
      for _ in $(seq 1 10); do
        if ! kill -0 "${pid}" >/dev/null 2>&1; then
          break
        fi
        sleep 0.2
      done
      if kill -0 "${pid}" >/dev/null 2>&1; then
        kill -9 "${pid}" >/dev/null 2>&1 || true
      fi
    fi
  done
}

cleanup() {
  if [[ -n "${HB_PID:-}" ]]; then
    if kill -0 "$HB_PID" >/dev/null 2>&1; then
      kill "$HB_PID" >/dev/null 2>&1 || true
      for _ in $(seq 1 10); do
        if ! kill -0 "$HB_PID" >/dev/null 2>&1; then
          wait "$HB_PID" 2>/dev/null || true
          HB_PID=""
          break
        fi
        sleep 0.2
      done
      kill -9 "$HB_PID" >/dev/null 2>&1 || true
      wait "$HB_PID" 2>/dev/null || true
    fi
    HB_PID=""
  fi
  stop_listener_port "${HB_PORT}"
  HB_LISTEN_PID=""
  HB_PORT=""
}
trap cleanup EXIT

wait_for_server() {
  local url="$1"
  local log_file="$2"
  local max_checks=120

  for _ in $(seq 1 "${max_checks}"); do
    if ! kill -0 "${HB_PID}" >/dev/null 2>&1; then
      echo "Server process exited early."
      if [[ -s "${log_file}" ]]; then
        tail -n 50 "${log_file}"
      fi
      return 1
    fi
    if curl -s "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done

  if [[ -s "${log_file}" ]]; then
    tail -n 50 "${log_file}"
  fi
  return 1
}

start_server() {
  local module="$1"
  local port="$2"
  local log_file="$3"

  ensure_module_dirs "${module}"
  ensure_port_available "${port}"

  echo "Starting hyperbricks for module ${module} on port ${port}..."
  ${HB_CMD} start -m "${module}" -p "${port}" > "${log_file}" 2>&1 &
  HB_PID=$!
  disown "${HB_PID}" >/dev/null 2>&1 || true
  HB_PORT="${port}"

  local url="http://localhost:${port}/"
  if ! wait_for_server "${url}" "${log_file}"; then
    echo "Server did not respond at ${url}"
    exit 1
  fi

  if command_exists lsof; then
    HB_LISTEN_PID=$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | awk '$1 ~ /^hyperbric/ {print $2; exit}' || true)
  fi
}

stop_server() {
  cleanup
  sleep 0.5
}

fetch_response() {
  local url="$1"
  local headers_file="$2"
  local body_file="$3"

  local response_headers
  response_headers=$(curl -s -D - -o "${body_file}" "${url}")
  echo "${response_headers}" | tr -d '\r' > "${headers_file}"
}

fetch_status() {
  local url="$1"
  curl -s -o /dev/null -w "%{http_code}" "${url}"
}

assert_status() {
  local url="$1"
  local expected="$2"
  local status
  status=$(fetch_status "${url}")
  if [[ "${status}" != "${expected}" ]]; then
    echo "Unexpected status for ${url}: expected ${expected}, got ${status}"
    exit 1
  fi
}

assert_body_contains_url() {
  local url="$1"
  local expected="$2"
  local body_file="$3"

  curl -s -o "${body_file}" "${url}"
  if ! grep -q "${expected}" "${body_file}"; then
    echo "Missing expected body content for ${url}"
    cat "${body_file}"
    exit 1
  fi
}

assert_route_ok() {
  local url="$1"
  local expected="$2"
  local body_file="$3"

  assert_status "${url}" "200"
  assert_body_contains_url "${url}" "${expected}" "${body_file}"
}

assert_routing_paths() {
  local base_url="$1"
  local body_file="$2"

  assert_route_ok "${base_url}/index" "HELLO WORLD!" "${body_file}"
  assert_route_ok "${base_url}/index.html" "HELLO WORLD!" "${body_file}"
  assert_route_ok "${base_url}/help" "HELP PAGE" "${body_file}"
  assert_route_ok "${base_url}/help.html" "HELP PAGE" "${body_file}"
  assert_status "${base_url}/missing" "404"
}

extract_rendered_at() {
  local body_file="$1"
  grep -o "Rendered at: [^<]*" "${body_file}" | head -n 1 || true
}

assert_header() {
  local headers_file="$1"
  local expected_header="$2"
  if ! grep -qi "^X-Test: ${expected_header}$" "${headers_file}"; then
    echo "Missing expected header: X-Test: ${expected_header}"
    cat "${headers_file}"
    exit 1
  fi
}

assert_cookie() {
  local headers_file="$1"
  local expected_cookie="$2"
  if ! grep -qi "^Set-Cookie: ${expected_cookie}$" "${headers_file}"; then
    echo "Missing expected Set-Cookie: ${expected_cookie}"
    cat "${headers_file}"
    exit 1
  fi
}

assert_body() {
  local body_file="$1"
  if ! grep -q "HELLO WORLD!" "${body_file}"; then
    echo "Missing expected body content"
    cat "${body_file}"
    exit 1
  fi
}

test_module_dev() {
  local module="$1"
  local port="$2"
  local expected_header="$3"
  local cookie_tag="$4"
  local base_url="http://localhost:${port}"
  local url="${base_url}/"
  local log_file="${LOG_DIR}/hyperbricks-${module}.log"
  local headers_file="${LOG_DIR}/hyperbricks-${module}.headers"
  local body_file="${LOG_DIR}/hyperbricks-${module}.body"
  local route_body_file="${LOG_DIR}/hyperbricks-${module}.route.body"

  start_server "${module}" "${port}" "${log_file}"

  fetch_response "${url}" "${headers_file}" "${body_file}"
  assert_header "${headers_file}" "${expected_header}"
  assert_cookie "${headers_file}" "session=${cookie_tag}; Path=/; HttpOnly"
  assert_cookie "${headers_file}" "prefs=${cookie_tag}; Path=/; Max-Age=31536000; SameSite=Lax"
  assert_body "${body_file}"

  assert_routing_paths "${base_url}" "${route_body_file}"

  if grep -q "Rendered at:" "${body_file}"; then
    echo "Unexpected cache marker in development mode"
    cat "${body_file}"
    exit 1
  fi

  stop_server
}

test_module_live() {
  local module="$1"
  local port="$2"
  local expected_header="$3"
  local cookie_tag="$4"
  local base_url="http://localhost:${port}"
  local url="${base_url}/"
  local log_file="${LOG_DIR}/hyperbricks-${module}.log"
  local headers_file="${LOG_DIR}/hyperbricks-${module}.headers"
  local body_file="${LOG_DIR}/hyperbricks-${module}.body"
  local route_body_file="${LOG_DIR}/hyperbricks-${module}.route.body"

  start_server "${module}" "${port}" "${log_file}"

  fetch_response "${url}" "${headers_file}" "${body_file}"
  assert_header "${headers_file}" "${expected_header}"
  assert_cookie "${headers_file}" "session=${cookie_tag}; Path=/; HttpOnly"
  assert_cookie "${headers_file}" "prefs=${cookie_tag}; Path=/; Max-Age=31536000; SameSite=Lax"
  assert_body "${body_file}"

  local first_rendered_at
  first_rendered_at=$(extract_rendered_at "${body_file}")
  if [[ -z "${first_rendered_at}" ]]; then
    echo "Missing cache marker in live mode response"
    cat "${body_file}"
    exit 1
  fi

  assert_routing_paths "${base_url}" "${route_body_file}"

  sleep 2

  fetch_response "${url}" "${headers_file}" "${body_file}"
  assert_header "${headers_file}" "${expected_header}"
  assert_cookie "${headers_file}" "session=${cookie_tag}; Path=/; HttpOnly"
  assert_cookie "${headers_file}" "prefs=${cookie_tag}; Path=/; Max-Age=31536000; SameSite=Lax"
  assert_body "${body_file}"

  local second_rendered_at
  second_rendered_at=$(extract_rendered_at "${body_file}")
  if [[ "${first_rendered_at}" != "${second_rendered_at}" ]]; then
    echo "Cache miss detected; rendered timestamp changed"
    echo "First: ${first_rendered_at}"
    echo "Second: ${second_rendered_at}"
    exit 1
  fi

  stop_server
}

test_module_dev "${MODULE_DEV}" "${PORT_DEV}" "headers-test-dev" "dev"
test_module_live "${MODULE_LIVE}" "${PORT_LIVE}" "headers-test-live" "live"

echo "PASS: All header/cookie/cache checks succeeded."
