# Response-cache HTTP benchmark

This benchmark compares fresh rendering, memory response caching and disk response
caching through the same running HyperBricks HTTP server. It uses the executable
fixture in `modules/response-cache-test`, a standard-library Go HTTP client and
real loopback TCP connections. It builds both executables before measuring,
copies the fixture into a temporary directory and removes the temporary module
and cache after graceful server shutdown. The source module is never started or
written by the runner. It leaves `HOME` unchanged.

From the repository root, run:

```sh
./benchmarks/response-cache/run.sh
```

The default matrix is 16 KiB and 256 KiB bodies, concurrency 1 and 16, all three
backends, and three rotating repeats: 36 trials. Each trial has 200 verified
warmup requests followed by 3,000 measured requests. For the first recorded
baseline, use a longer run:

```sh
./benchmarks/response-cache/run.sh -requests 10000 -warmup 1000 -repeats 3
```

A small lifecycle/verification smoke run (not a performance baseline) is:

```sh
./benchmarks/response-cache/run.sh -requests 40 -warmup 20 -repeats 1 -sizes 16384 -concurrency 1,4
```

Run while other builds, tests and load generators are stopped. The fixture sets
server `GOMAXPROCS=4` and the runner defaults to client `GOMAXPROCS=4`. Both
processes share the host, so these numbers include competition for CPU, the
loopback network stack and the scheduler. No Caddy process is involved. macOS and
Linux are supported; the fixed server configuration requires at least four
logical CPUs.

## Workload and validation

The three routes use one inherited template with identical output:

| Backend | Route | Policy |
| --- | --- | --- |
| Fresh | `/bench/fresh` | `nocache: true`; render each response |
| Memory | `/bench/mem` | `cache: {storage: mem, expire: 10m}` |
| Disk | `/bench/disk` | `cache: {storage: disk, expire: 10m}` |

The `blocks` query parameter selects the repetition count of the exact 16-byte
string `0123456789abcdef`: 1,024 blocks produce 16,384 bytes and 16,384 blocks
produce 262,144 bytes. This is a controlled rendering/caching comparison, not a
representative complex application or database workload.

Every warmup and measured response must have HTTP status 200, the expected
Content-Length and plain-text media type, no render errors, and exactly the
expected bytes **and** SHA-256. The client sends no conditional headers and
requests identity encoding. It requires uncompressed HTTP/1.1 keep-alive.
Cacheable responses must retain the ETag and internal render/expiry timestamps
seen during warmup; fresh responses must have no internal cache metadata.

After each trial, outside timing, the runner checks disk file count, length and
SHA-256 against the disk variants warmed so far. This catches accidental changes
that make the supposed disk route use memory, or make the memory route write
disk files. Shutdown must exit cleanly and remove the runtime cache namespace.
Any response, storage or lifecycle failure invalidates the run and suppresses
aggregate comparisons. Do not publish performance figures from an invalid run.

Each trial has a new client connection pool. One excluded seed request ensures
the cache is populated before concurrent warmup. The warmup then establishes
connections and warms application and filesystem caches. The server remains
alive across trials. Backend order rotates by repeat and case; case order also
rotates across repeats. With three repeats, each backend occupies every position
within its size/concurrency group once.

**Disk results measure warm files and warm operating-system page cache**, not
cold physical-disk reads. The ten-minute cache lifetime is deliberately longer
than this baseline; if a long customized run expires an entry during a trial,
the metadata check invalidates that trial. Source `.cache` and `generated-cache`
directories are excluded when staging the fixture.

## Timing and statistics

Each closed-loop worker sends its next request after it has read and verified
the previous response. There is no fixed arrival rate, client-side request queue
or coordinated-omission correction.

- Throughput is verified measured requests divided by batch wall time. That time
  includes client scheduling, response reading, byte comparisons and SHA-256
  verification. Client body validation consumes CPU and can limit throughput.
- Response latency starts immediately before `http.Client.Do` and ends after
  the complete body is read and closed. It excludes subsequent byte/hash checks.
  It includes any wait for a client connection and network/server time.
- Every successful measured request contributes one raw nanosecond sample. The
  average is the arithmetic mean. Trial p50/p95/p99 use nearest rank:
  `sorted[ceil(p * sample_count) - 1]`. No samples or outliers are dropped.
- The comparison table reports median trial throughput, minimum and maximum,
  and spread `(max - min) / median * 100`. Latency columns are **medians of the
  individual trial statistics**, including p95 and p99. Percentiles are neither
  pooled nor averaged. For an even number of repeats, medians use the mean of the
  two central values. Ranges describe observed variation, not confidence bounds.
- No application-level retries occur. Go's HTTP transport may transparently
  retry an idempotent request after a reused connection fails. Each trial records
  new/reused connection acquisitions; unexpected new connections deserve review.

These are comparisons on one local host, not production capacity claims. The
fixture does no database or upstream HTTP work. File sizes are recorded as an
inventory; this benchmark does **not** measure process RSS, heap usage or the
memory occupied by the operating-system page cache. Existing Go microbenchmarks
may accompany this report as a separate measurement; do not mix their
nanoseconds/op or retained-body counters with HTTP latency or RSS claims.

## Results and provenance

Each run creates a new UTC timestamp directory under `results/` containing:

- `result.json`: complete options, environment, provenance, validation outcomes,
  raw trials, every successful latency sample and aggregate summaries.
- `report.md`: readable comparison, raw-trial table and measurement limitations.
- `server.log`: stdout/stderr from the isolated HyperBricks process.

Results are ignored by Git. Curated baseline snapshots can be reviewed and kept
under `baselines/`. The runner source fingerprint excludes both directories so
recording results does not change its own identity. JSON is compact to keep
360,000 latency samples reasonably small.

The first recorded comparison is [the 2026-10-05 baseline](baselines/20261005/README.md).
Freeze fixture and runner sources before recording a baseline. After changing
either, rerun the baseline or explicitly identify the changed documentation-only
hash; do not present a previous source fingerprint as the current one.

Provenance includes the Git revision, branch, dirty status, tracked diff hash,
content hashes of relevant untracked inputs, SHA-256/build information for both
binaries, and per-file manifests for the fixture and runner. The staged fixture
must match its source manifest before the server is launched. Environment
records OS/version, CPU, logical CPU count, Go versions, execution parallelism
and explicitly inherited `GOGC`, `GOMEMLIMIT` and `GODEBUG` tuning. It does not
dump arbitrary environment variables.

Common options:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-requests` | `3000` | Measured requests per trial |
| `-warmup` | `200` | Excluded verified requests per trial |
| `-repeats` | `3` | Complete matrix repetitions |
| `-sizes` | `16384,262144` | Distinct body sizes, positive multiples of 16 |
| `-concurrency` | `1,16` | Distinct closed-loop worker counts |
| `-client-procs` | `4` | Client execution parallelism |
| `-request-timeout` | `10s` | Complete individual response deadline |
| `-startup-timeout` | `30s` | Isolated server readiness deadline |
| `-output` | Unique `results/<UTC timestamp>` | New directory; existing paths are rejected |

For an already-built server, build the runner separately, then call it with
`-binary /absolute/path/to/hyperbricks -repo /absolute/path/to/checkout` and the
same flags. The recorded binary hashes identify exactly what was measured.

Runner checks:

```sh
go test ./benchmarks/response-cache
go vet ./benchmarks/response-cache
```
