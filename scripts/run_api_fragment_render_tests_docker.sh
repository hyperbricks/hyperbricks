#!/bin/bash
set -euo pipefail
set +m

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${REPO_ROOT}/test/dedicated/docker/docker-compose.yml"
COMPOSE_PROJECT="hyperbricks-api-tests-$$"
TEST_SERVER_LOG="${TEST_SERVER_LOG:-/tmp/hyperbricks-api-render-test-server.log}"
TEST_SERVER_PID=""
TEST_RUN_DIR=""
STACK_STARTED=false
RUN_LOCK="${TMPDIR:-/tmp}/hyperbricks-api-render-tests-${UID}.lock"
LOCK_ACQUIRED=false

cd "${REPO_ROOT}"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker CLI is not on PATH. Install Docker or open a new terminal after installation, then verify 'docker version'." >&2
  exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose is unavailable. Verify 'docker compose version' before running these tests." >&2
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  echo "Docker is not reachable. Start Docker and verify 'docker info' before running these tests." >&2
  exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required to generate temporary Docker test credentials." >&2
  exit 1
fi

# Keep credentials process-local: Docker Compose and the YAML fixtures receive
# the same fresh values without writing them into the repository.
export POSTGRES_DB="${POSTGRES_DB:-postgres}"
export POSTGRES_USER="${POSTGRES_USER:-postgres}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(openssl rand -hex 24)}"
export PGRST_JWT_SECRET="${PGRST_JWT_SECRET:-$(openssl rand -hex 32)}"
export PGADMIN_DEFAULT_PASSWORD="${PGADMIN_DEFAULT_PASSWORD:-$(openssl rand -hex 24)}"

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

  for entry in ${listeners}; do
    local cmd="${entry%%:*}"
    local pid="${entry##*:}"
    echo "Port ${port} is in use by ${cmd} (pid ${pid}). Wait for the other run or stop the process yourself, then re-run." >&2
  done
  return 1
}

cleanup() {
  if [[ "${STACK_STARTED}" == "true" ]]; then
    docker compose -p "${COMPOSE_PROJECT}" -f "${COMPOSE_FILE}" down -v >/dev/null 2>&1 || true
  fi

  if [[ -n "${TEST_SERVER_PID:-}" ]] && kill -0 "${TEST_SERVER_PID}" >/dev/null 2>&1; then
    stop_pid "${TEST_SERVER_PID}"
  fi
  TEST_SERVER_PID=""
  if [[ -n "${TEST_RUN_DIR}" ]]; then
    rm -rf "${TEST_RUN_DIR}"
  fi
  if [[ "${LOCK_ACQUIRED}" == "true" ]]; then
    rm -f "${RUN_LOCK}/pid"
    rmdir "${RUN_LOCK}" || true
  fi
}

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

if ! mkdir "${RUN_LOCK}" 2>/dev/null; then
  echo "Another API render test run owns ${RUN_LOCK}. Wait for it to finish. If a run was forcibly killed, remove this lock only after checking that it has stopped." >&2
  exit 1
fi
LOCK_ACQUIRED=true
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
printf '%s\n' "$$" > "${RUN_LOCK}/pid"

ensure_port_available 8090
ensure_port_available "${POSTGREST_PORT:-3000}"
TEST_RUN_DIR=$(mktemp -d "${TMPDIR:-/tmp}/hyperbricks-api-render-tests.XXXXXX")
echo "Building local API test server..."
go build -o "${TEST_RUN_DIR}/api-test-server" ./cmd/testing
echo "Starting local API test server..."
"${TEST_RUN_DIR}/api-test-server" >"${TEST_SERVER_LOG}" 2>&1 &
TEST_SERVER_PID=$!
wait_for_url "http://localhost:8090/echo/query?ready=1" "local API test server" "${TEST_SERVER_PID}"

echo "Starting PostgREST docker stack..."
STACK_STARTED=true
docker compose -p "${COMPOSE_PROJECT}" -f "${COMPOSE_FILE}" up -d postgres server
wait_for_url "http://127.0.0.1:${POSTGREST_PORT:-3000}/" "PostgREST docker stack"

echo "Running Docker YAML API_RENDER and API_FRAGMENT_RENDER tests..."
go test -v ./test/dedicated -run '^Test_All_Dedicated_YAML_Tests$' -args -directory="./yaml-api-tests/"
