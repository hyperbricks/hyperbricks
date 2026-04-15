#!/bin/bash
set -euo pipefail
set +m

echo "Running docker CACHE AND PATH MARKER test..."

MODULE="markers-test"
PORT="8085"
HB_CMD="${HB_CMD:-go run ./cmd/hyperbricks}"
LOG_FILE="/tmp/hyperbricks-marker-tests.log"

ROUTES=(
  "http://localhost:${PORT}/test_001"
  "http://localhost:${PORT}/test_002"
  "http://localhost:${PORT}/test_003"
)

EXPECTED_FILES=(
  "test/dedicated/cache-path-marker-tests/expected_result-001.html"
  "test/dedicated/cache-path-marker-tests/expected_result-002.html"
  "test/dedicated/cache-path-marker-tests/expected_result-003.html"
)

PROCESS_PID=""

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

ensure_module_dirs() {
  local module="$1"
  local module_dir="modules/${module}"

  mkdir -p \
    "${module_dir}/static" \
    "${module_dir}/resources" \
    "${module_dir}/templates" \
    "${module_dir}/rendered" \
    "./bin/plugins"
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
}

stop_listener_port() {
  local port="$1"

  if ! command_exists lsof; then
    return 0
  fi

  local listeners
  listeners=$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $1 ":" $2}' || true)
  if [[ -z "${listeners}" ]]; then
    return 0
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
  if [[ -n "${PROCESS_PID:-}" ]] && kill -0 "${PROCESS_PID}" >/dev/null 2>&1; then
    kill "${PROCESS_PID}" >/dev/null 2>&1 || true
    for _ in $(seq 1 10); do
      if ! kill -0 "${PROCESS_PID}" >/dev/null 2>&1; then
        wait "${PROCESS_PID}" 2>/dev/null || true
        PROCESS_PID=""
        break
      fi
      sleep 0.2
    done
    if [[ -n "${PROCESS_PID:-}" ]] && kill -0 "${PROCESS_PID}" >/dev/null 2>&1; then
      kill -9 "${PROCESS_PID}" >/dev/null 2>&1 || true
      wait "${PROCESS_PID}" 2>/dev/null || true
    fi
  fi
  PROCESS_PID=""
  stop_listener_port "${PORT}"
}
trap cleanup EXIT

wait_for_server() {
  local url="$1"
  local log_file="$2"
  local max_checks=120

  for _ in $(seq 1 "${max_checks}"); do
    if ! kill -0 "${PROCESS_PID}" >/dev/null 2>&1; then
      echo "Hyperbricks exited before it became ready."
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

  echo "Timed out waiting for ${url}"
  if [[ -s "${log_file}" ]]; then
    tail -n 50 "${log_file}"
  fi
  return 1
}

fetch_route() {
  local url="$1"
  local log_file="$2"

  if ! curl -fsS "${url}"; then
    echo "Failed to get a response from hyperbricks at ${url}."
    if [[ -s "${log_file}" ]]; then
      tail -n 50 "${log_file}"
    fi
    return 1
  fi
}

normalize_html() {
  echo "$1" | tr -s '[:space:]' ' ' | sed 's/[[:space:]]*$//g'
}

ensure_module_dirs "${MODULE}"
ensure_port_available "${PORT}"

echo "Starting hyperbricks..."
${HB_CMD} start -m "${MODULE}" -p "${PORT}" > "${LOG_FILE}" 2>&1 &
PROCESS_PID=$!
disown "${PROCESS_PID}" >/dev/null 2>&1 || true

echo "Hyperbricks process started with PID ${PROCESS_PID}."
echo "Waiting for hyperbricks to be ready..."
wait_for_server "${ROUTES[0]}" "${LOG_FILE}"

all_tests_passed=true

for idx in "${!ROUTES[@]}"; do
  route="${ROUTES[$idx]}"
  expected_file="${EXPECTED_FILES[$idx]}"

  echo "Testing route: ${route}"

  response="$(fetch_route "${route}" "${LOG_FILE}")"
  expected_result="$(cat "${expected_file}")"

  expected_result_normalized="$(normalize_html "${expected_result}")"
  response_normalized="$(normalize_html "${response}")"

  if [[ "${response_normalized}" == "${expected_result_normalized}" ]]; then
    echo "PASS: HTML structure matches the expected template from ${expected_file}."
  else
    echo "FAIL: HTML structure does not match the expected template from ${expected_file}."
    echo "Expected HTML structure (normalized):"
    echo "${expected_result_normalized}"
    echo "Actual HTML structure (normalized):"
    echo "${response_normalized}"
    all_tests_passed=false
  fi
done

echo "Waiting for exiting hyperbricks..."
cleanup
sleep 0.5

if [[ "${all_tests_passed}" == "true" ]]; then
  echo "All tests passed successfully!"
else
  echo "Some tests failed."
  exit 1
fi

if command_exists lsof && lsof -nP -iTCP:"${PORT}" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "Port ${PORT} is in use."
  exit 1
fi

echo "Everything cleaned"
