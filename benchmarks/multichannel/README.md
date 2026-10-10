# Multichannel API benchmark

This runner exercises a real, isolated HyperBricks process over loopback HTTP.
A page fans out to catalog, stock and shipping APIs through three nested
`api_render` instances. Three `api_fragment_render` routes expose those services
separately. Concurrent closed-loop users rotate through all four routes. Every
request has a unique identifier, which must appear in the complete expected
response of every participating service. The upstream fixture waits 1 ms per
call and returns JSON with 32 KiB catalog, 4 KiB stock and 1 KiB shipping payloads.
The complete payload is rendered and independently checked. This is a synthetic
service workload, not a claim about real-world commerce or database behavior.

Each repeat runs both successful traffic and mixed traffic (every third request
asks stock to return HTTP 503). The page retains all channel bodies and maps the
stock failure to HTTP 503. Policies are mounted explicitly on inherited instances.
The independently constructed response contract checks status, exact body,
content type, no-store policy, absence of response cookies and rendered cache
metadata. Incorrect responses invalidate the run; failures are not discarded.

## Build and run

Build the server from a clean committed checkout and retain that executable.
The runner requires embedded clean Git revision metadata and rejects race builds.
Build the runner once and use that same executable for both measurements:

```sh
go build -buildvcs=true -trimpath -o /tmp/hyperbricks-baseline ./cmd/hyperbricks
go build -trimpath -o /tmp/multichannel-runner ./benchmarks/multichannel
mkdir -p internal-benchmarks/multichannel
/tmp/multichannel-runner -binary /tmp/hyperbricks-baseline \
  -requests 2000 -warmup 200 -repeats 7 -concurrency 16 \
  -output internal-benchmarks/multichannel/reference
/tmp/multichannel-runner -binary /tmp/hyperbricks-baseline \
  -requests 2000 -warmup 200 -repeats 7 -concurrency 16 \
  -output internal-benchmarks/multichannel/repeat
/tmp/multichannel-runner -compare internal-benchmarks/multichannel/reference/result.json \
  internal-benchmarks/multichannel/repeat/result.json
```

Existing output directories are never overwritten. Run on macOS or Linux with
`ps` and `uname`, permission to bind local ports and inspect owned process RSS.
Stop other tests and builds first. Each run owns its temporary module, upstream
server and application process. Startup and warmup are excluded. SIGTERM stops
the application; a failed or forced shutdown invalidates the run. A port selected
before startup can be taken by another process; the runner never kills that
process, and response verification prevents accepting unrelated content.

Options default to 1,200 measured requests, 120 warmup requests, five repeats,
eight users and `-settle 1s`. Use the same settings for comparisons. Both processes
use four Go execution slots; the server environment pins GOGC=100 and
GOMEMLIMIT=off. Unset inherited GOGC/GOMEMLIMIT/GODEBUG for the runner. The runtime
profile uses live mode, no route caching, no Dashboard and error-level logging.
Expected failed API diagnostics remain part of the mixed workload cost.

## Evidence and comparison

`result.json` retains raw per-request nanosecond latencies, trial elapsed time,
50 ms RSS samples (KiB), RSS before/peak/after each trial, exact server and runner
binary SHA-256 and Go build settings, normalized fixture hash, embedded runner
source hash, host identifier hash, OS/kernel and CPU count. `server.log` retains
application output. Keep both binaries with the results. The embedded source
hash covers the runner's Go sources, including the fixture and comparator.

Latency includes the full response read and close, excluding subsequent response
validation. Throughput includes validation and worker scheduling. RSS is sampled
in a separate goroutine using `ps`; sampling overhead is part of the measurement.
Settled RSS is taken after the configured idle interval without forcing GC.
Peak RSS is the maximum **sampled** value, not a guarantee of the process peak.

The comparator requires complete validated trial matrices, clean shutdown,
at least five repeats, identical runner/fixture/host/settings/trial order and
compatible Go build flags. It recomputes throughput and p95 from raw samples.
It rejects missing, invalid, dirty, raced or incompatible evidence. It checks
throughput, p95 and settled RSS independently for success and mixed traffic.

The regression threshold is the greater of a **5% policy floor** and twice the
baseline's observed full range divided by its median. The floor avoids treating
tiny changes as meaningful; twice the observed range is a conservative noise
allowance, not a statistical confidence interval. A baseline or candidate range
above 20% makes that metric inconclusive and rejects the comparison. All metrics
are printed even when one is inconclusive. Exit 0 means no measured regression
exceeds the limits; exit 2 means invalid, inconclusive or regressed evidence.
Calibrate with independent A/A runs before using this as a release gate. A noisy
A/A is evidence to improve measurement conditions, not permission to widen limits.

For release calibration, reserve a quiet Linux environment with fixed CPU
affinity and memory limits. Record the OS, kernel, CPU topology, effective CPU
set, memory limit and CPU throttling counters alongside the results. The
comparator checks host identity and CPU count, but cannot detect an affinity
change or competing host workloads by itself. A container shares its host's
kernel and physical CPUs; fixed limits do not reserve exclusive physical cores.

Before measuring different server revisions, declare three independent A/A
runs (A, B and C) with the same frozen binaries and options. Compare A/B and
A/C; both must pass. Keep all runs, including failed and inconclusive attempts.
Do not remove slow trials or disable error logging, response validation or RSS
sampling to obtain a pass. If the environment or runner changes, establish a
new complete baseline rather than comparing across that change. Keep binaries,
raw results and environment records outside Git in `internal-benchmarks/`.

Client, upstream and server share a host; contention, temperature and scheduling
remain confounders. Hashing the host name cannot prove unchanged hardware or
background load. This is a warm fixed-size workload; it does not establish peak
production capacity, tail latency under open-loop overload, cancellation cost,
heap retention or absence of leaks. Increasing RSS can include allocator/GC
behavior; a longer soak and heap profiles are needed to diagnose leaks. No
performance improvement is implied by a successful A/A run.
