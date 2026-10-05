# SSR baseline — 2026-10-05

This run measures the [nested SSR fixture](../../../../modules/ssr-proof-hyperbricks/README.md)
through HyperBricks' HTTP server. Each request renders a 938-byte HTML page with
a unique request ID in three positions. The route has `nocache: true` and
`Cache-Control: no-store`.

## Reproduce

From the repository root, with other builds and load generators stopped:

```sh
./benchmarks/ssr/run.sh -requests 50000 -warmup 5000 -repeats 3 \
  -concurrency 1,16,64 -client-procs 4
```

The recorded run completed from **15:32:26 to 15:32:37 UTC** on an Apple M3 with
eight logical CPUs, macOS 26.6.2, arm64 and Go 1.26.1. Server and verifying client
each used four Go execution slots and shared the machine. The server used the
fixture's default live profile, 30-second timeouts, keep-alive, a request limit
of 1,000,000/second and info logging.

The measured server was built at
`61a7d4fdacefe817a9c0d5db99993d28cb6cc190` on `release/v1.3.0-beta`, with the new
benchmark still uncommitted. The JSON records this development snapshot's dirty
state, tracked diff hash, untracked source hashes, binary identities, and fixture
and runner manifests. Both executables were built with `go build -trimpath`
before measurement. The source manifests and retained executable hashes were
verified after the run.

## Results

Each row summarizes three trials. Throughput is the median verified requests per
second; latency columns are medians of the corresponding trial percentiles.

| Concurrent requests | Median requests/s | Observed requests/s range | p95 latency | p99 latency |
| ---: | ---: | ---: | ---: | ---: |
| 1 | 23,667 | 23,500–23,950 | 0.048 ms | 0.082 ms |
| 16 | 95,490 | 95,041–102,994 | 0.396 ms | 0.601 ms |
| 64 | 100,596 | 84,660–106,656 | 1.680 ms | 2.496 ms |

The observed throughput range relative to the median was **1.9%**, **8.3%**, and
**21.9%** respectively. The concurrency-64 result varied substantially; its
median alone is insufficient to establish a small performance change. No
outliers were removed. These ranges are descriptive, not confidence intervals.

This run uses fixed request counts. Single-worker trials lasted about 2.1
seconds; concurrency-16 and concurrency-64 trials lasted about 0.47–0.59 seconds.
For closer comparisons, increase the request count, repeat the pair in reversed
order, and review the full trial ranges.

The run is valid: **nine trials, 450,000 measured responses, 45,000 excluded
warmup responses and four excluded preflight responses**. Every response passed
the complete HTML, status, length, content-type, SHA-256, request-isolation and
cache-header checks. Preflights also exercised escaping and repeated input.
All measured connection acquisitions reused established connections. Shutdown
was clean, with zero response-cache body files.

The [complete report](report.md) includes every trial and its latency summaries.
The [original JSON](result.json) retains all 450,000 latency samples, the exact
canonical HTML, validation evidence, source manifests and build provenance.
The [server log](server.log) is preserved unchanged.

## Comparing another run

Use the [SSR runner and comparison instructions](../../README.md) to measure two
server executables with one fixed runner and fixture. To verify this archive's
comparison path:

```sh
python3 benchmarks/compare-ssr.py \
  benchmarks/ssr/baselines/20261005/result.json \
  benchmarks/ssr/baselines/20261005/result.json
```

That self-comparison must produce three rows with zero deltas. It checks the
saved evidence and comparison calculations; it adds no performance measurement.

This archive records the runner executable's hash. Retain the executable as
well to measure another server against this exact run on the same machine.
Without the identical runner, fixture and recorded environment, measure a new
pair. The comparator rejects incompatible inputs.

These are warm, local HTTP measurements of a small nested-template workload.
The client shares CPU with the server and includes HTML generation and response
validation in throughput. Startup, preflight and warmup are excluded. The
workload includes no Caddy, database or upstream API, and measures no process
memory. Use the recorded conditions when interpreting the results.
