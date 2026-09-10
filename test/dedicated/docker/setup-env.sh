#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required to generate local test credentials." >&2
  exit 1
fi

if [[ -e "${ENV_FILE}" ]]; then
  echo "Keeping existing ${ENV_FILE}. Remove it first to generate new values."
  exit 0
fi

umask 077
cat >"${ENV_FILE}" <<EOF
POSTGRES_DB=postgres
POSTGRES_USER=postgres
POSTGRES_PASSWORD=$(openssl rand -hex 24)
PGRST_JWT_SECRET=$(openssl rand -hex 32)
PGADMIN_DEFAULT_EMAIL=admin@example.invalid
PGADMIN_DEFAULT_PASSWORD=$(openssl rand -hex 24)
POSTGREST_PORT=3000
PGADMIN_PORT=15432
EOF

echo "Created ${ENV_FILE} with local test-only credentials."
