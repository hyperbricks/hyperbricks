#!/bin/bash
set -euo pipefail
set +m

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${REPO_ROOT}/test/dedicated/docker/docker-compose.yml"
TEST_SERVER_LOG="${TEST_SERVER_LOG:-/tmp/hyperbricks-api-render-test-server.log}"
TEST_SERVER_PID=""

cd "${REPO_ROOT}"

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

stop_pid() {
  local pid="$1"

  kill "${pid}" >/dev/null 2>&1 || true
  for _ in $(seq 1 20); do
    if ! kill -0 "${pid}" >/dev/null 2>&1; then
      wait "${pid}" 2>/dev/null || true
      return 0
    fi
    sleep 0.2
  done
  kill -9 "${pid}" >/dev/null 2>&1 || true
  wait "${pid}" 2>/dev/null || true
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
    if [[ "${cmd}" == "main" ]]; then
      echo "Port ${port} is held by a stale API test server (pid ${pid}); stopping it..."
      stop_pid "${pid}"
    else
      echo "Port ${port} is in use by ${cmd} (pid ${pid}). Stop it and re-run."
      blocked="yes"
    fi
  done

  if [[ -n "${blocked}" ]]; then
    exit 1
  fi
}

cleanup() {
  docker compose -f "${COMPOSE_FILE}" down -v >/dev/null 2>&1 || true

  if [[ -n "${TEST_SERVER_PID:-}" ]] && kill -0 "${TEST_SERVER_PID}" >/dev/null 2>&1; then
    stop_pid "${TEST_SERVER_PID}"
  fi
  TEST_SERVER_PID=""
}
trap cleanup EXIT

wait_for_url() {
  local url="$1"
  local label="$2"
  local monitored_pid="${3:-}"
  local max_checks=120

  for _ in $(seq 1 "${max_checks}"); do
    if [[ -n "${monitored_pid}" ]] && ! kill -0 "${monitored_pid}" >/dev/null 2>&1; then
      echo "${label} exited before it became ready."
      if [[ -s "${TEST_SERVER_LOG}" ]]; then
        tail -n 50 "${TEST_SERVER_LOG}"
      fi
      return 1
    fi
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done

  echo "Timed out waiting for ${label} at ${url}."
  if [[ -s "${TEST_SERVER_LOG}" ]]; then
    tail -n 50 "${TEST_SERVER_LOG}"
  fi
  return 1
}

echo "Starting local API test server..."
ensure_port_available 8090
go run ./cmd/testing/main.go >"${TEST_SERVER_LOG}" 2>&1 &
TEST_SERVER_PID=$!
wait_for_url "http://localhost:8090/echo/query?ready=1" "local API test server" "${TEST_SERVER_PID}"

echo "Starting PostgREST docker stack..."
docker compose -f "${COMPOSE_FILE}" down -v >/dev/null 2>&1 || true
docker compose -f "${COMPOSE_FILE}" up -d postgres server
wait_for_url "http://localhost:3000/" "PostgREST docker stack"

echo "Running Docker YAML API_RENDER and API_FRAGMENT_RENDER tests..."
go test -v ./test/dedicated -run '^Test_All_Dedicated_YAML_Tests$' -args -directory="./yaml-api-tests/"
