#!/bin/sh
set -e

HB_HOME=${HB_HOME:-/opt/hyperbricks}
HB_USER=${HB_USER:-deploy}
HB_GROUP=${HB_GROUP:-deploy}

if [ -n "${TZ:-}" ] && [ -f "/usr/share/zoneinfo/${TZ}" ]; then
  cp "/usr/share/zoneinfo/${TZ}" /etc/localtime
  printf '%s\n' "${TZ}" > /etc/timezone
fi

mkdir -p "${HB_HOME}/bin" "${HB_HOME}/bin/plugins" "${HB_HOME}/deploy" "${HB_HOME}/.cache" "${HB_HOME}/go"
chown -R "${HB_USER}:${HB_GROUP}" \
  "${HB_HOME}/bin" \
  "${HB_HOME}/bin/plugins" \
  "${HB_HOME}/deploy" \
  "${HB_HOME}/.cache" \
  "${HB_HOME}/go" 2>/dev/null || true

if [ ! -f "${HB_HOME}/deploy.hyperbricks.yaml" ] && [ -f "/etc/hyperbricks/deploy.hyperbricks.yaml" ]; then
  cp /etc/hyperbricks/deploy.hyperbricks.yaml "${HB_HOME}/deploy.hyperbricks.yaml"
  chown "${HB_USER}:${HB_GROUP}" "${HB_HOME}/deploy.hyperbricks.yaml" 2>/dev/null || true
fi

if [ -z "${HB_DEPLOY_SECRET:-}" ]; then
  echo "warning: HB_DEPLOY_SECRET is not set" >&2
fi

if [ "${HB_BUILD_SOURCE:-checkout}" = "checkout" ]; then
  export HYPERBRICKS_LOCAL_PATH=/opt/hyperbricks-source
fi

cd "${HB_HOME}"
exec su-exec "${HB_USER}:${HB_GROUP}" "${HB_HOME}/bin/hyperbricks" deploy-daemon
