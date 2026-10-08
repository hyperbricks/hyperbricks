#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
build_dir=$(mktemp -d "${TMPDIR:-/tmp}/hyperbricks-cache-benchmark-build.XXXXXX")
runner_pid=
stop_runner() {
  if [ -n "$runner_pid" ]; then
    kill -TERM "$runner_pid" 2>/dev/null || :
    wait "$runner_pid" 2>/dev/null || :
    runner_pid=
  fi
}
trap 'stop_runner; rm -rf -- "$build_dir"' EXIT
trap 'stop_runner; exit 130' INT
trap 'stop_runner; exit 143' TERM

cd "$repo"
printf '%s\n' 'Building HyperBricks and benchmark runner before measurement...'
go build -trimpath -o "$build_dir/hyperbricks" ./cmd/hyperbricks
go build -trimpath -o "$build_dir/response-cache" ./benchmarks/response-cache
"$build_dir/response-cache" -repo "$repo" -binary "$build_dir/hyperbricks" "$@" &
runner_pid=$!
wait "$runner_pid"
runner_pid=
