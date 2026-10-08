# Response cache fixture

A runnable regression and benchmark module for per-route memory and disk response
caching. All responses are plain text. The ordinary probes use the built-in
template engine's current timestamp, so they require no plugin, Goja runtime or
upstream service. The optional API probe connects only when requested.

## Run the module

From the repository root, using the current checkout:

```sh
go run ./cmd/hyperbricks doctor -m response-cache-test --strict
go run ./cmd/hyperbricks start -m response-cache-test
```

The default listener is port **8179**. Use `--port 8189` to choose another port.
`GET /healthz` and `GET /` return `ok` without caching.

The package uses live mode, a ten-minute default cache lifetime, four Go execution
slots, disabled rate limiting and info-level logging. Watchers and developer
interfaces are disabled. Template beautification is disabled so response bytes
remain unchanged. The checked-in directory placeholders allow the module to run
from a clean checkout or a temporary copy.

## Automated checks and baseline

Run the complete HTTP/CLI matrix through the repository module runner:

```sh
python3 scripts/test_modules.py --module response-cache-test
```

This builds the current checkout once, runs disposable module copies on free
ports, and checks expiry, purge, cleanup, limits, mode bypass, API fragments,
archive exclusions and invalid configuration. It also runs with
`./tests.sh --with-modules`. No active development module is modified.

To reuse a binary or retain successful test logs:

```sh
python3 scripts/test_response_cache_module.py --binary /path/to/hyperbricks --keep-workdir
```

The separate [response-cache benchmark](../../benchmarks/response-cache/README.md)
compares the identical workloads below and records environment, source hashes,
raw trials and latency distributions. Follow its fixed baseline command for
comparisons; the smoke checks test correctness without measuring performance.

## Route policies

The probes share one inherited template in
[probes.hyperbricks.yaml](hyperbricks/probes.hyperbricks.yaml). Each response has
the form `rendered_at=<UnixNano> variant=<value>`. The `variant` query parameter
defaults to `default` and participates in the existing request cache key.

| Route | Component | Cache configuration | Default live behavior |
| --- | --- | --- | --- |
| `/probe/default` | Fragment | Omitted | Memory, package lifetime of 10m |
| `/probe/scalar` | Fragment | `cache: 1s` | Memory, 1s |
| `/probe/mem` | Fragment | `{storage: mem, expire: 1s}` | Memory, 1s |
| `/probe/expire-only` | Fragment | `{expire: 1s}` | Memory, 1s |
| `/probe/empty` | Fragment | `{}` | Memory, package lifetime of 10m |
| `/probe/disk` | Fragment | `{storage: disk, expire: 1s}` | Disk, 1s |
| `/probe/disk-inherit` | Fragment | `{storage: disk}` | Disk, package lifetime of 10m |
| `/probe/zero-mem` | Fragment | `{storage: mem, expire: 0s}` | Fresh on every request |
| `/probe/zero-scalar` | Fragment | `cache: 0s` | Fresh on every request |
| `/probe/zero-disk` | Fragment | `{storage: disk, expire: 0s}` | Fresh on every request |
| `/probe/nocache` | Fragment | Disk, 10m, plus `nocache: true` | Fresh on every request |
| `/probe/page-mem` | Hypermedia | `{storage: mem, expire: 1s}` | Memory, 1s |
| `/probe/page-disk` | Hypermedia | `{storage: disk, expire: 1s}` | Disk, 1s |

Each ordinary probe sets `X-Cache-Fixture: response-cache-test` and
`Cache-Control: no-store`. The latter controls browser/proxy caching and does not
disable HyperBricks' internal response cache.

For a cacheable response, inspect `ETag`, `X-Hyperbricks-Rendered-At` and
`X-Hyperbricks-Cache-Expires-At`. Two requests within the lifetime should keep the
same body timestamp and cache headers. The request ID can change on a cache hit.
The response's rendered timestamp changes after expiry or purge.

```sh
curl -i 'http://127.0.0.1:8179/probe/disk?variant=one'
curl -i 'http://127.0.0.1:8179/probe/disk?variant=one'
sleep 2
curl -i 'http://127.0.0.1:8179/probe/disk?variant=one'
curl -i 'http://127.0.0.1:8179/probe/disk?variant=two'
```

## Package profiles

Each profile is a complete package configuration. Stop the current process before
starting the next profile on the same port:

```sh
go run ./cmd/hyperbricks start -m response-cache-test --config package.development.hyperbricks.yaml
```

| Package file | Mode and behavior |
| --- | --- |
| `package.hyperbricks.yaml` | Live; package cache 10m; disk maximum 256MiB of response bodies and 10,000 entries |
| `package.development.hyperbricks.yaml` | Development; every request renders fresh despite route cache settings |
| `package.debug.hyperbricks.yaml` | Debug; every request renders fresh despite route cache settings |
| `package.disabled.hyperbricks.yaml` | Live with package `cache: 0s`; disables both backends even for a positive route lifetime |
| `package.custom.hyperbricks.yaml` | Live; stores response cache files under this module's `generated-cache/responses/` |
| `package.budget.hyperbricks.yaml` | Live; disk maximum 4,096 body bytes and two entries; cleanup every 100ms |

The other profiles run cleanup every **200ms**, deliberately faster than the
runtime's normal one-minute default to make cleanup observable during smoke
tests. Cleanup never extends a response's lifetime. With the budget profile,
request three variants of `/probe/disk` quickly to exercise entry eviction. A
16KiB benchmark response exceeds that profile's disk budget and must be served
fresh without a memory fallback.

## Purge and disk lifetime

Keep the server running and use another terminal:

```sh
curl -i 'http://127.0.0.1:8179/probe/disk-inherit?variant=one'
curl -i 'http://127.0.0.1:8179/probe/disk-inherit?variant=two'
go run ./cmd/hyperbricks cache purge -m response-cache-test --route probe/disk-inherit
curl -i 'http://127.0.0.1:8179/probe/disk-inherit?variant=one'
go run ./cmd/hyperbricks cache purge -m response-cache-test --all
```

Route purge removes every cached request variant for that route. Purge-all
invalidates both memory and disk entries. The command contacts the running
instance through its private local control connection; it does not depend on an
application HTTP route. If several processes run this module, select the intended
one using the `--instance` value reported by the command.

The default disk directory is `.cache/responses/<runtime-id>/` inside the module.
Files are disposable: expired responses cannot be served, periodic cleanup
reclaims obsolete files, graceful shutdown removes the runtime namespace, and
responses are **not reused across server restarts**. Startup/maintenance reclaim
abandoned namespaces only when their recorded process is proven absent.

Deployment packages and runtime snapshots exclude `.cache` by default. Custom
directory exclusion reads **`package.hyperbricks.yaml`**, so starting with
`--config package.custom.hyperbricks.yaml` alone does not make a subsequent build
exclude `generated-cache`. For a custom-directory build, make that cache path
the default package setting in the build source. The automated packaging test
does this in its disposable copy before checking ZIP and HRA contents.
Both locations are ignored by this fixture's `.gitignore`; Git ignores and
deployment exclusions remain separate mechanisms. See the repository's
[deployment documentation](../../docs/DEPLOY.md) for packaging behavior and
[migration guide](../../docs/MIGRATION.md) for the newly effective scalar route
duration.

## Deterministic benchmark workload

[benchmarks.hyperbricks.yaml](hyperbricks/benchmarks.hyperbricks.yaml) is separate
from the dynamic probes. `/bench/mem`, `/bench/disk` and `/bench/fresh` inherit the
same template and return identical bytes for a given `blocks` query value:

```text
0123456789abcdef repeated blocks times, with no added whitespace or newline
```

| Query | Exact response size |
| --- | ---: |
| Omitted or `blocks=1024` | 16,384 bytes (16KiB) |
| `blocks=16384` | 262,144 bytes (256KiB) |

Memory and disk routes use ten-minute lifetimes; the fresh route uses
`nocache: true`. Use the default profile for comparisons. The budget, disabled,
development and debug profiles deliberately change cache eligibility or storage
behavior and therefore test different conditions.

```sh
curl -sS 'http://127.0.0.1:8179/bench/mem?blocks=1024' | wc -c
curl -sS 'http://127.0.0.1:8179/bench/disk?blocks=1024' | wc -c
curl -sS 'http://127.0.0.1:8179/bench/fresh?blocks=1024' | wc -c
```

## Optional API exclusion probe

`/probe/api` is an `API_FRAGMENT_RENDER`. It always makes a fresh upstream request
and bypasses rendered-response caching. Its default upstream is
`http://127.0.0.1:8180/cache-probe`; set `HB_CACHE_TEST_UPSTREAM` to another base URL
before starting HyperBricks. The `variant` query parameter is forwarded.

A controlled upstream should answer JSON such as `{"call": 1, "variant": "one"}`,
incrementing `call` on each request and echoing the received `variant` query value.
The probe renders those upstream data fields as `upstream_call=1 variant=one`.
All ordinary probes and benchmark routes work without this upstream.
