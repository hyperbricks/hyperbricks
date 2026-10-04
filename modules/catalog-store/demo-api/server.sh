#!/bin/sh
set -eu

exec python3 server.py \
  --port "${CATALOG_API_PORT:-4319}" \
  --site-url "http://127.0.0.1:${HB_SERVER_PORT}"
