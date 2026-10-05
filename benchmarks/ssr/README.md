# HyperBricks SSR benchmark

Measure fresh server-side HTML rendering through HyperBricks' HTTP server. The
runner uses the repository's [SSR fixture](../../modules/ssr-proof-hyperbricks/README.md),
checks every response against independent expected HTML, and records the raw
samples and build provenance needed to compare later runs.

The workload composes a landing page from nested templates and trees. Each
request supplies a unique `rid` query value that appears in three places: a
quoted attribute, a feature-card paragraph, and a definition-list value. There
are no plugins, upstream API calls, or database queries in the measured route.

## Run

Use macOS or Linux with at least four logical CPUs and the Go version required
by the repository's `go.mod`. From the repository root:

```sh
./benchmarks/ssr/run.sh
```

The wrapper builds HyperBricks and the Go benchmark runner before measurement.
The runner copies the fixture into a temporary directory, starts its own server
on an available loopback port, performs response preflights, runs the trials,
and shuts that server down. It leaves the source module unchanged. Interrupting
the wrapper stops its runner and server before removing temporary build files.

The defaults are 10,000 measured requests and 1,000 excluded warmup requests per
trial, three repeats, and concurrency levels of 1, 16, and 64. That gives nine
trials, 90,000 measured responses, and 9,000 warmup responses, plus four excluded
preflight requests. For a longer run:

```sh
./benchmarks/ssr/run.sh -requests 50000 -warmup 5000 -repeats 3
```

Stop other builds and load generators before recording results. Keep the
machine, fixture, runtime settings, client settings, and trial sizes consistent
when comparing runs.

## Workload and Runtime Settings

The measured request is `GET /?rid=<request-id>`, served by the fixture's
`index` route. Its `nocache: true` policy disables HyperBricks' rendered-response
cache, and `Cache-Control: no-store` declares the client-side HTTP policy.
Every measured request renders the page again. Templates and the runtime are
warmed before measurement.

The runner uses `package.hyperbricks.yaml` with these settings:

| Setting | Value |
| --- | --- |
| Runtime mode | `live` |
| Server Go execution parallelism | `GOMAXPROCS=4` |
| Request, write, and idle timeouts | 30 seconds each |
| HTTP keep-alive | Enabled |
| Request limiter | Enabled; 1,000,000 requests/second and a 1,000,000-request burst |
| HTML beautification | Disabled |
| Measured server logging | Info |
| Client Go execution parallelism | 4 by default |

The fixture's separate `package.raw.hyperbricks.yaml` profile is not used by
this runner. Startup and build time are excluded from the HTTP measurements.
The benchmark contract is the complete HTTP response and workload.

The four preflight requests exercise ordinary values, HTML escaping, and a
repeated value. Expected escaping follows Go's HTML-template behavior. Warmup
and measured requests then use distinct fixed-width, 38-byte ASCII identifiers
containing a run nonce, trial number, phase, and request index. This keeps the
measured response size consistent while checking request isolation.

## Response Validation

The client requests complete HTTP/1.1 responses with identity encoding. It uses
no proxy or conditional requests and rejects redirects. Every warmup and
measured response must satisfy the runner's contract:

- HTTP 200 and an HTML content type.
- The complete expected body, including the correct request value in all three
  positions, with matching length, bytes, and SHA-256.
- `Cache-Control: no-store`, zero reported HyperBricks render errors, and no
  internal rendered-response cache metadata.
- An uncompressed HTTP/1.1 response that permits connection reuse.

A failed request, wrong body, or invalid response makes the run invalid. The
runner retains failure information and does not publish a successful aggregate
comparison from an invalid run. The expected body is defined independently of
the server response; the first response is not accepted as the expected output.

## Measurement Method

Workers issue requests in a closed loop: each worker waits for a complete
response before starting its next request. Concurrency therefore counts
outstanding HTTP requests, not CPU cores or Go execution slots. Trial order
rotates the concurrency settings across repeats. Each trial gets a new client
connection pool, which its excluded warmup prepares for the measured requests.

Latency starts immediately before `client.Do` and ends after the complete body
has been read and closed. It excludes the subsequent byte and hash checks.
Throughput is verified responses divided by measured batch wall time, which
includes request-ID and expected-body generation, client scheduling, response
reads, and validation work.

The JSON retains every successful latency sample in nanoseconds. Percentiles
use nearest rank, and the report summarizes trial throughput with a median,
minimum, maximum, and range relative to the median. No outliers are removed.
Reported latency summaries compare the individual trials; they are not pooled
latency distributions. Small-sample ranges are descriptive, not confidence
intervals.

The client and server share the same machine. CPU scheduling, the local network
stack, client verification, other processes, and thermal conditions can affect
results. Four Go execution slots do not reserve four physical cores. These
measurements describe warm rendering of this small template workload; they do
not establish production capacity, cold-start behavior, or performance of
applications with database and API work. No process RSS or heap measurement is
made. For memory and disk response-cache measurements, use the separate
[response-cache benchmark](../response-cache/README.md).

## Results and Provenance

By default, each run creates a new timestamped directory under
`benchmarks/ssr/results/`. Existing output directories are never overwritten.

| File | Contents |
| --- | --- |
| `result.json` | Run validity, options, environment, source and binary provenance, response checks, trials, and raw latency samples |
| `report.md` | Trial summaries, throughput ranges, latency statistics, and measurement method |
| `server.log` | Output from the isolated HyperBricks server |

Keep the complete directory when retaining or sharing a measurement. A report
without its validity, fixture identity, binary identity, and measurement
settings is insufficient for a reproducible comparison.

The [2026-10-05 baseline](baselines/20261005/README.md) records a complete run
with its results, environment, and interpretation limits.

## Options

Pass these options after `run.sh`:

| Option | Default | Purpose |
| --- | --- | --- |
| `-requests` | `10000` | Measured requests per trial |
| `-warmup` | `1000` | Excluded warmup requests per trial |
| `-repeats` | `3` | Repeats of the concurrency matrix |
| `-concurrency` | `1,16,64` | Distinct concurrent-worker counts |
| `-client-procs` | `4` | Client Go execution parallelism |
| `-request-timeout` | `10s` | Timeout for each complete client response |
| `-startup-timeout` | `30s` | Maximum wait for server readiness |
| `-output` | New timestamped directory | Result directory; must not already exist |

Request counts, warmup counts, repeats, timeouts, and parallelism must be
positive. Each concurrency must be no greater than the request and warmup
counts. The server's four-slot setting belongs to the fixture and stays fixed.

## Compare HyperBricks Builds

Build the runner once when testing existing HyperBricks executables:

```sh
mkdir -p .cache/benchmarks
go build -trimpath -o .cache/benchmarks/ssr-runner ./benchmarks/ssr

.cache/benchmarks/ssr-runner \
  -repo "$PWD" \
  -binary /absolute/path/to/hyperbricks-version-a \
  -output benchmarks/ssr/results/version-a

.cache/benchmarks/ssr-runner \
  -repo "$PWD" \
  -binary /absolute/path/to/hyperbricks-version-b \
  -output benchmarks/ssr/results/version-b
```

The standalone runner also accepts `-module`, defaulting to
`modules/ssr-proof-hyperbricks` relative to `-repo`. It requires the fixture's
configuration and response contract. Use the same fixture and options for
both binaries, preserve the build provenance of each executable, and verify
that both runs are valid before comparing their trial distributions. The
runner records the supplied executable separately from the checkout used for
the fixture and benchmark source. Repeating comparisons in alternating order
helps reveal variation caused by machine conditions.

Compare the saved runs with Python 3.9 or newer:

```sh
python3 benchmarks/compare-ssr.py \
  benchmarks/ssr/results/version-a/result.json \
  benchmarks/ssr/results/version-b/result.json
```

The first argument is the reference and the second is the candidate. The command
reads both files without changing them and writes Markdown to stdout. Add
`--json` for structured output. Exit status **0** means the measurements are
valid and comparable, regardless of the performance change; status **2** means
an input is invalid, incomplete, or incompatible. There is no automatic
performance-regression gate.

Use the **same frozen runner executable** for both measurements. The comparator
requires its exact binary SHA-256, matching runner and fixture source manifests,
and the same canonical workload. Keep those sources unchanged between runs,
including their READMEs, which are part of the fingerprints. Rebuilding
unchanged runner code may produce a different binary hash because Go embeds VCS
metadata. If the executed runner differs, measure a new pair with one runner.

The recorded hardware, OS/kernel, Go versions, compiler flags, runtime tuning,
request counts, warmup, concurrency, deadlines, execution slots, and trial order
must also match. Run both measurements on the same physical machine; matching
recorded CPU and OS strings cannot establish that identity or equal background
load. Server binary hashes, revisions, and dirty state may differ. The server's
identity comes from its executable hash and embedded build settings; the
runner checkout revision identifies the fixture/tooling checkout separately.

The comparator checks complete response validation and preflights, clean server
shutdown, and the full trial matrix. It recalculates throughput and latency
statistics from counts, elapsed times, and raw samples rather than trusting
saved aggregate summaries. Its report shows median throughput, observed trial
ranges, and median trial average/p50/p95/p99 latency for both runs. Changes use
`(candidate / reference - 1) × 100`: positive throughput is higher and negative
latency is lower. Review the trial ranges before interpreting small changes.
