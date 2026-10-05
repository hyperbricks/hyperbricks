# HyperBricks SSR HTTP benchmark

Run: 2026-10-05 15:32:26Z to 2026-10-05 15:32:37Z. **Valid: true.**

Host: Apple M3, darwin 26.6.2 (arm64); 8 logical CPUs. Client Go go1.26.1, GOMAXPROCS 4; server Go go1.26.1, GOMAXPROCS 4. Both processes share this machine.

Server revision: `61a7d4fdacefe817a9c0d5db99993d28cb6cc190`, modified="true", SHA-256 `3e6e08fa04c9dc345edfdad81fda3c843c5dde5b9d17b743c7b027c7bcd9692d`. Runner checkout: `61a7d4fdacefe817a9c0d5db99993d28cb6cc190` on `release/v1.3.0-beta`, dirty=true. The binary's embedded revision identifies the measured server; the runner checkout can differ for a prebuilt server.

Fixed profile `package.hyperbricks.yaml`, info logging. Each trial: 5000 excluded verified warmup requests then 50000 measured verified requests, 3 repeats. Concurrency: [1 16 64]. Each measured HTML body is 938 bytes with a unique 38-byte request ID repeated at three positions. Excluded preflight requests: 4, valid=true.

## Comparison across repeats

Throughput spread = (maximum − minimum) / median × 100. Latency columns are medians of individual trial averages or percentiles; percentiles are not pooled or averaged. Ranges are descriptive, not confidence intervals.

| Workers | Trials | Median req/s | Min–max req/s | Spread | Avg ms | p50 ms | p95 ms | p99 ms |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 3 | 23667.4 | 23500.1–23950.3 | 1.9% | 0.040 | 0.038 | 0.048 | 0.082 |
| 16 | 3 | 95489.8 | 95041.2–102994.2 | 8.3% | 0.164 | 0.129 | 0.396 | 0.601 |
| 64 | 3 | 100596.3 | 84660.2–106655.6 | 21.9% | 0.632 | 0.478 | 1.680 | 2.496 |

## Raw trials

| Order | Repeat | Workers | Valid | Verified | Elapsed s | req/s | Avg ms | p50 ms | p95 ms | p99 ms | New/reused connections |
| ---: | ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1 | 1 | true | 50000 | 2.112611 | 23667.4 | 0.040 | 0.039 | 0.048 | 0.082 | 0/50000 |
| 2 | 1 | 16 | true | 50000 | 0.526088 | 95041.2 | 0.165 | 0.129 | 0.399 | 0.601 | 0/50000 |
| 3 | 1 | 64 | true | 50000 | 0.468799 | 106655.6 | 0.596 | 0.449 | 1.561 | 2.265 | 0/50000 |
| 4 | 2 | 16 | true | 50000 | 0.485464 | 102994.2 | 0.152 | 0.123 | 0.356 | 0.525 | 0/50000 |
| 5 | 2 | 64 | true | 50000 | 0.497036 | 100596.3 | 0.632 | 0.478 | 1.680 | 2.496 | 0/50000 |
| 6 | 2 | 1 | true | 50000 | 2.087655 | 23950.3 | 0.040 | 0.038 | 0.048 | 0.079 | 0/50000 |
| 7 | 3 | 64 | true | 50000 | 0.590596 | 84660.2 | 0.751 | 0.531 | 2.056 | 3.673 | 0/50000 |
| 8 | 3 | 1 | true | 50000 | 2.127651 | 23500.1 | 0.040 | 0.038 | 0.049 | 0.084 | 0/50000 |
| 9 | 3 | 16 | true | 50000 | 0.523616 | 95489.8 | 0.164 | 0.129 | 0.396 | 0.639 | 0/50000 |

## Method

- One isolated HyperBricks server uses the fixture's default live profile: four Go execution slots, 30-second transport timeouts, keep-alive and a 1,000,000 requests/second and burst limiter. Explicit CLI info logging excludes per-request debug output.
- The index route renders fresh per request with nocache:true and Cache-Control:no-store. Nested templates and trees interpolate the unique query rid at three positions. Every complete response is compared byte-for-byte and by SHA-256 with an independently authored canonical HTML expectation.
- Each timed/warmup rid is 38 ASCII bytes: ssr-<16lowerhex>-<4digittrial>-<w|m>-<10digitindex>. A random per-run nonce, trial sequence, phase and reservation index prevent reuse across the run; all measured responses have the same byte length.
- Four excluded preflight requests check ordinary and escaping-sensitive request IDs and repeated input. Each trial then has its own excluded warmup and a new connection pool; the server remains alive across the complete run. Concurrency order rotates each repeat.
- Real loopback HTTP/1.1 TCP with keep-alive, identity encoding, no proxy, conditional requests or application retries. Go Transport can retry an idempotent GET after a reused connection failure; new/reused connection acquisitions are recorded.
- Closed-loop workers send the next request after verification. Throughput is verified requests divided by measured batch wall time, including request-ID/expected-body generation, byte/hash verification and scheduling. Client buffers are allocated before timing.
- Latency begins immediately before client.Do and ends after the complete response body is read and closed, before byte/hash validation. It includes connection waiting and network/server work. There is no arrival-rate scheduling or coordinated-omission correction.
- Every successful measured response contributes one raw nanosecond sample in reservation order. Trial p50/p95/p99 use nearest rank sorted[ceil(p*N)-1], average is arithmetic, and no outliers are dropped. Across repeats report medians of trial statistics and observed throughput ranges.
- The verifying client and server share one host, so client CPU, the scheduler and local network stack can limit throughput. No Caddy, upstream service, process RSS, heap profile or production-capacity measurement is included.

Clean server shutdown: true. Response-cache body files after run: 0.

Fixture SHA-256: `430b3af72cd2af86aaa9548c5d12318c1aaa44fe0b8cdab34c51b7606884c182`. Runner source SHA-256: `6d66a6e6c5522f7e8bc62f81ba3c56b49b7b2da51dca9426510eecf81e588e6c`. Runner binary SHA-256: `f38870e4bda42809f1d5562d254f391d51e84b78fda14995bfad10cf7ef41394`.

`result.json` retains every measured response latency, validation evidence, request-ID prefixes, exact canonical template and source/binary provenance. `server.log` retains the isolated server output. Build, startup, preflight and warmup are excluded from measured trials.
