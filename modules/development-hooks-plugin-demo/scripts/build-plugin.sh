#!/bin/sh
set -eu
if [ "${HOOKS_DEMO_FAIL_PLUGIN_BUILD:-0}" = "1" ]; then
  echo "Requested plugin build failure (HOOKS_DEMO_FAIL_PLUGIN_BUILD=1)" >&2
  exit 1
fi
# The existing plugin CLI resolves modules/ and bin/plugins/ from its cwd.
project_root=$(CDPATH= cd -- "$HB_MODULE_ROOT/../.." && pwd)
cd "$project_root"
export HYPERBRICKS_LOCAL_PATH="$project_root"
python3 -c 'import pathlib, uuid; pathlib.Path("modules/development-hooks-plugin-demo/plugins/hook-greeting/1.0.0/build-id.txt").write_text(str(uuid.uuid4()), encoding="utf-8")'
exec "$HB_EXECUTABLE" plugin build hook-greeting@1.0.0 --module development-hooks-plugin-demo
