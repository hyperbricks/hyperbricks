#!/bin/sh
set -eu
# Stay in the foreground. exec lets the managed process directly own the API.
exec python3 server.py --port "${HOOKS_DEMO_API_PORT:-4331}" \
  --site-port "${HB_SERVER_PORT:-4330}"
