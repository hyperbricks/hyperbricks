#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -f "${SCRIPT_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${SCRIPT_DIR}/.env"
  set +a
fi

: "${PGRST_JWT_SECRET:?Run ./setup-env.sh or set PGRST_JWT_SECRET}"

API_URL="${API_URL:-http://127.0.0.1:${POSTGREST_PORT:-3000}}"
TEST_USERNAME="${TEST_USERNAME:-testuser-$(date +%s)}"
TEST_PASSWORD="${TEST_PASSWORD:-$(openssl rand -hex 16)}"
TEST_EMAIL="${TEST_EMAIL:-${TEST_USERNAME}@example.invalid}"

base64url() {
  openssl base64 -A | tr '/+' '_-' | tr -d '='
}

generate_superuser_jwt() {
  local header payload signature
  header=$(printf '%s' '{"alg":"HS256","typ":"JWT"}' | base64url)
  payload=$(printf '%s' '{"role":"postgres"}' | base64url)
  signature=$(printf '%s' "${header}.${payload}" | openssl dgst -sha256 -hmac "${PGRST_JWT_SECRET}" -binary | base64url)
  printf '%s' "${header}.${payload}.${signature}"
}

SUPERUSER_JWT=$(generate_superuser_jwt)

echo "Checking the privileged test function..."
TEST_USER_RESPONSE=$(curl --fail-with-body --silent --show-error -X POST "${API_URL}/rpc/dummy_admin" \
  -H "Authorization: Bearer ${SUPERUSER_JWT}" \
  -H "Content-Type: application/json" \
  -d '{}')
echo "Privileged function response: ${TEST_USER_RESPONSE}"

echo "Creating an isolated test user..."
CREATE_USER_RESPONSE=$(curl --fail-with-body --silent --show-error -i -X POST "${API_URL}/rpc/create_user" \
  -H "Authorization: Bearer ${SUPERUSER_JWT}" \
  -H "Content-Type: application/json" \
  -d "{\"p_username\":\"${TEST_USERNAME}\",\"p_password\":\"${TEST_PASSWORD}\",\"p_email\":\"${TEST_EMAIL}\"}")
echo "Create-user response: ${CREATE_USER_RESPONSE}"

echo "Logging in as the test user..."
LOGIN_RESPONSE=$(curl --fail-with-body --silent --show-error -X POST "${API_URL}/rpc/login_user" \
  -H "Content-Type: application/json" \
  -d "{\"p_password\":\"${TEST_PASSWORD}\",\"p_username\":\"${TEST_USERNAME}\"}")
USER_JWT=$(printf '%s' "${LOGIN_RESPONSE}" | sed -E 's/.*"([^"]+)".*/\1/')

if [[ -z "${USER_JWT}" || "${USER_JWT}" == "${LOGIN_RESPONSE}" ]]; then
  echo "The login response did not contain a JWT." >&2
  exit 1
fi

echo "Creating a task as the test user..."
CREATE_TASK_RESPONSE=$(curl --fail-with-body --silent --show-error -X POST "${API_URL}/tasks" \
  -H "Authorization: Bearer ${USER_JWT}" \
  -H "Content-Type: application/json" \
  -d '{"title":"Complete PostgreSQL RLS setup","completed":false}')
echo "Create-task response: ${CREATE_TASK_RESPONSE:-created}"

echo "Fetching tasks visible to the test user..."
GET_TASKS_RESPONSE=$(curl --fail-with-body --silent --show-error -X GET "${API_URL}/tasks" \
  -H "Authorization: Bearer ${USER_JWT}")
echo "Task response: ${GET_TASKS_RESPONSE}"

echo "All RLS fixture checks completed successfully."
