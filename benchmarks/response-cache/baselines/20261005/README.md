# Response-cache baseline — 2026-10-05

This is the first recorded HTTP baseline for the executable
[response-cache fixture](../../../../modules/response-cache-test/README.md).
It compares fresh rendering, memory cache hits and disk cache hits in the same
HyperBricks process. It establishes a reference for later changes; it does not
measure improvement over an earlier release.

## Reproduce

From the repository root, with other builds and load generators stopped:

```sh
python3 scripts/test_modules.py --module response-cache-test
./benchmarks/response-cache/run.sh -requests 10000 -warmup 1000 -repeats 3
```

The recorded run completed from 13:24:58 to 13:26:12 UTC on an Apple M3 with eight
logical CPUs, macOS 26.6.2, arm64 and Go 1.26.1. Server and verifying client each
used `GOMAXPROCS=4` and shared the host. Source revision was
`4c6e9256b27b19e6244655d632240418ef00aee4` on `release/v1.3.0-beta`, with this new
fixture and benchmark still uncommitted. The JSON records the dirty state,
tracked diff hash, untracked source hashes, binary hashes and complete fixture
and runner manifests. Those fixture and runner files were checked against the
recorded hashes after measurement.

The run is valid: **36 trials, 360,000 measured responses and 36,000 excluded
warmup responses**, including one seed request per trial. Every response was
validated for status, length, content type, exact bytes and SHA-256; cache
metadata was checked against the selected policy. Every measured connection
acquisition reused an established connection. Actual disk bodies were checked
after each trial. Shutdown was clean, with zero remaining cache namespaces.

## Measured throughput

Each value is the median requests per second across three trials. All backends
produce exactly the same deterministic template output for the selected size.

| Response size | Concurrent requests | Fresh rendering | Memory cache | Disk cache |
| --- | ---: | ---: | ---: | ---: |
| 16 KiB | 1 | 7,514 | 20,669 | 13,496 |
| 16 KiB | 16 | 23,098 | 78,813 | 47,301 |
| 256 KiB | 1 | 1,086 | 3,890 | 3,393 |
| 256 KiB | 16 | 3,745 | 11,527 | 14,734 |

Both caching paths outperformed fresh rendering for this fixture. Memory was
faster in three of the four groups. The 256 KiB disk group at concurrency 16 had
a higher median than memory, but its trials ranged from **11,338 to 14,758
requests/s**, a **23.2%** range relative to its median. Preserve that variation
when comparing future runs; three trials do not establish that disk is generally
faster for large responses. Every other group's observed range was at most 8.4%.
No outliers were removed, and these ranges are not confidence intervals.

The [complete report](report.md) includes each trial, throughput ranges, average
latency and p50/p95/p99. [The original JSON](result.json) retains all 360,000
nanosecond latency samples, checks and provenance. [The server log](server.log)
is also preserved unchanged from the run.

## What these numbers measure

- Real loopback HTTP/1.1 with keep-alive, identity encoding and a local Go client;
  build, startup, seed requests and warmup are excluded from measured trials.
- Warm application caches and warm operating-system filesystem pages. The disk
  results do not measure cold physical-disk reads.
- A simple deterministic text template, with no database, upstream service or
  Caddy. These results are a local comparison, not production capacity.
- Throughput includes client scheduling and byte/hash validation. Latency ends
  after reading and closing the body, before that validation. The client shares
  CPU with the server and can constrain throughput, especially for larger bodies.
- No process RSS or heap measurement. Two disk bodies totaling 278,528 bytes were
  present before shutdown; that inventory is not a process-memory measurement.

Keep the fixture, parallelism, payloads and validation contract fixed for future
comparisons. Use the [runner documentation](../../README.md) for the complete
method, flags and artifact format.

## Companion handler microbenchmark

The existing `BenchmarkResponseCacheBackendHit` was also run with four Go
execution slots, three one-second repetitions and a 16 KiB response. It uses a
discard response writer and excludes TCP. These numbers are a separate view of
handler overhead, not HTTP latency.

| Backend/configuration | Median ns/op | Median B/op | Allocs/op | Retained body bytes per cache entry |
| --- | ---: | ---: | ---: | ---: |
| Default memory | 971.6 | 384 | 20 | 16,384 |
| Scalar memory | 1,003 | 384 | 20 | 16,384 |
| Mapping memory | 1,063 | 384 | 20 | 16,384 |
| Disk | 21,641 | 1,121 | 28 | 0 |

Disk entries retain no response-body bytes in the application's cache entry.
Metadata and transient allocations still consume process memory, and the
operating system can retain file pages in RAM. The zero counter is not zero RSS.
The [raw output](handler-microbench.txt) and [structured summary](handler-microbench.json)
include the exact command and all repetitions.
