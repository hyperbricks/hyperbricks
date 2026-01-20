#!/bin/sh
set -e

HB_HOME=${HB_HOME:-/opt/hyperbricks}
HB_USER=${HB_USER:-deploy}
HB_GROUP=${HB_GROUP:-deploy}
HB_USE_OPENRC=${HB_USE_OPENRC:-0}
HB_AUTHORIZED_KEYS_FILE=${HB_AUTHORIZED_KEYS_FILE:-/etc/hyperbricks/authorized_keys}

mkdir -p /run/openrc /run/sshd /var/run/sshd
: > /run/openrc/softlevel

if [ -n "${TZ:-}" ] && [ -f "/usr/share/zoneinfo/${TZ}" ]; then
  cp "/usr/share/zoneinfo/${TZ}" /etc/localtime
  printf '%s\n' "${TZ}" > /etc/timezone
fi

mkdir -p "${HB_HOME}/bin" "${HB_HOME}/bin/plugins" "${HB_HOME}/deploy" "${HB_HOME}/.ssh" "${HB_HOME}/.cache" "${HB_HOME}/go"
chown -R "${HB_USER}:${HB_GROUP}" \
  "${HB_HOME}/bin" \
  "${HB_HOME}/bin/plugins" \
  "${HB_HOME}/deploy" \
  "${HB_HOME}/.ssh" \
  "${HB_HOME}/.cache" \
  "${HB_HOME}/go" 2>/dev/null || true

if [ -f "${HB_AUTHORIZED_KEYS_FILE}" ]; then
  cp "${HB_AUTHORIZED_KEYS_FILE}" "${HB_HOME}/.ssh/authorized_keys"
  chown "${HB_USER}:${HB_GROUP}" "${HB_HOME}/.ssh/authorized_keys" 2>/dev/null || true
  chmod 600 "${HB_HOME}/.ssh/authorized_keys"
fi

if [ ! -f "${HB_HOME}/deploy.hyperbricks" ] && [ -f "/etc/hyperbricks/deploy.hyperbricks" ]; then
  cp /etc/hyperbricks/deploy.hyperbricks "${HB_HOME}/deploy.hyperbricks"
  chown "${HB_USER}:${HB_GROUP}" "${HB_HOME}/deploy.hyperbricks" 2>/dev/null || true
fi

if [ -f "${HB_HOME}/.ssh/authorized_keys" ]; then
  chmod 700 "${HB_HOME}/.ssh"
  chmod 600 "${HB_HOME}/.ssh/authorized_keys"
  chown "${HB_USER}:${HB_GROUP}" "${HB_HOME}/.ssh" "${HB_HOME}/.ssh/authorized_keys" 2>/dev/null || true
fi

if [ -z "${HB_DEPLOY_SECRET:-}" ]; then
  echo "warning: HB_DEPLOY_SECRET is not set" >&2
fi

if [ ! -f /etc/ssh/ssh_host_rsa_key ]; then
  ssh-keygen -A
fi

if ! /usr/sbin/sshd -t; then
  echo "error: sshd config test failed" >&2
  exit 1
fi
/usr/sbin/sshd -e

if [ "${HB_USE_OPENRC}" = "1" ] && [ -x /etc/init.d/hyperbricks-deploy ]; then
  if ! rc-service hyperbricks-deploy start; then
    echo "warning: OpenRC failed; starting Hyperbricks directly" >&2
    exec su -s /bin/sh -c "cd ${HB_HOME} && ${HB_HOME}/bin/hyperbricks start --deploy-remote" "${HB_USER}"
  fi
  echo "OpenRC started hyperbricks-deploy; set HB_USE_OPENRC=0 for foreground logs." >&2
  exec tail -f /dev/null
fi

exec su -s /bin/sh -c "cd ${HB_HOME} && ${HB_HOME}/bin/hyperbricks start --deploy-remote" "${HB_USER}"
