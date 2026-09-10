#!/usr/bin/env bash
set -Eeuo pipefail

: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"
: "${PGRST_JWT_SECRET:?PGRST_JWT_SECRET is required}"

psql \
  --username "${POSTGRES_USER}" \
  --dbname "${POSTGRES_DB}" \
  --set=ON_ERROR_STOP=1 \
  --set=database_name="${POSTGRES_DB}" \
  --set=jwt_secret="${PGRST_JWT_SECRET}" \
  --file=/docker-init/init.sql
