# Response-cache HTTP benchmark

Run: 2026-10-05 13:24:58Z to 2026-10-05 13:26:12Z. **Valid: true.**

Host: Apple M3, darwin 26.6.2 (arm64); 8 logical CPUs. Client Go go1.26.1, GOMAXPROCS 4; server Go go1.26.1, GOMAXPROCS 4. Both processes share this machine.

Git: `4c6e9256b27b19e6244655d632240418ef00aee4` on `release/v1.3.0-beta`, dirty=true. The JSON records exact status, binary hashes, build settings and source-file manifests.

Each trial: 1000 excluded verified warmup requests, then 10000 measured verified requests; 3 repeats. Sizes: [16384 262144] bytes; concurrency: [1 16].

## Comparison across repeats

Throughput columns summarize whole trials. Spread = (maximum − minimum) / median × 100. Latency columns are medians of the individual trial averages or percentiles, not pooled percentiles or averages of percentiles. These small-sample ranges are descriptive, not confidence intervals.

| Bytes | Workers | Backend | Trials | Median req/s | Min–max req/s | Spread | Avg ms | p50 ms | p95 ms | p99 ms |
| ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 16384 | 1 | fresh | 3 | 7514.0 | 7255.1–7529.5 | 3.7% | 0.125 | 0.116 | 0.193 | 0.266 |
| 16384 | 1 | mem | 3 | 20669.5 | 18946.4–20688.7 | 8.4% | 0.041 | 0.040 | 0.048 | 0.069 |
| 16384 | 1 | disk | 3 | 13496.3 | 13469.7–13550.1 | 0.6% | 0.066 | 0.064 | 0.085 | 0.117 |
| 16384 | 16 | fresh | 3 | 23097.8 | 23068.8–23376.2 | 1.3% | 0.680 | 0.579 | 1.494 | 2.045 |
| 16384 | 16 | mem | 3 | 78812.6 | 77554.1–79734.6 | 2.8% | 0.193 | 0.165 | 0.418 | 0.595 |
| 16384 | 16 | disk | 3 | 47301.2 | 45459.1–48199.9 | 5.8% | 0.326 | 0.272 | 0.746 | 1.086 |
| 262144 | 1 | fresh | 3 | 1086.1 | 1073.6–1098.2 | 2.3% | 0.816 | 0.799 | 0.949 | 1.046 |
| 262144 | 1 | mem | 3 | 3889.6 | 3867.5–3916.4 | 1.3% | 0.156 | 0.155 | 0.170 | 0.199 |
| 262144 | 1 | disk | 3 | 3393.2 | 3381.8–3405.4 | 0.7% | 0.193 | 0.190 | 0.213 | 0.268 |
| 262144 | 16 | fresh | 3 | 3744.9 | 3665.9–3785.7 | 3.2% | 4.135 | 3.542 | 9.108 | 12.876 |
| 262144 | 16 | mem | 3 | 11527.2 | 11441.6–11572.3 | 1.1% | 1.246 | 1.084 | 2.688 | 3.594 |
| 262144 | 16 | disk | 3 | 14733.9 | 11338.1–14757.5 | 23.2% | 0.944 | 0.833 | 1.991 | 2.592 |

## Raw trials

| Order | Repeat | Bytes | Workers | Backend | Valid | Verified | Elapsed s | req/s | Avg ms | p50 ms | p95 ms | p99 ms | New/reused connections |
| ---: | ---: | ---: | ---: | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1 | 16384 | 1 | fresh | true | 10000 | 1.378349 | 7255.1 | 0.129 | 0.120 | 0.200 | 0.266 | 0/10000 |
| 2 | 1 | 16384 | 1 | mem | true | 10000 | 0.527805 | 18946.4 | 0.045 | 0.040 | 0.073 | 0.121 | 0/10000 |
| 3 | 1 | 16384 | 1 | disk | true | 10000 | 0.742408 | 13469.7 | 0.066 | 0.064 | 0.088 | 0.117 | 0/10000 |
| 4 | 1 | 16384 | 16 | mem | true | 10000 | 0.125416 | 79734.6 | 0.191 | 0.164 | 0.405 | 0.581 | 0/10000 |
| 5 | 1 | 16384 | 16 | disk | true | 10000 | 0.207469 | 48199.9 | 0.319 | 0.265 | 0.728 | 1.007 | 0/10000 |
| 6 | 1 | 16384 | 16 | fresh | true | 10000 | 0.427785 | 23376.2 | 0.671 | 0.582 | 1.430 | 1.850 | 0/10000 |
| 7 | 1 | 262144 | 1 | disk | true | 10000 | 2.936512 | 3405.4 | 0.193 | 0.189 | 0.214 | 0.275 | 0/10000 |
| 8 | 1 | 262144 | 1 | fresh | true | 10000 | 9.207534 | 1086.1 | 0.816 | 0.789 | 0.949 | 1.046 | 0/10000 |
| 9 | 1 | 262144 | 1 | mem | true | 10000 | 2.553344 | 3916.4 | 0.155 | 0.153 | 0.166 | 0.190 | 0/10000 |
| 10 | 1 | 262144 | 16 | fresh | true | 10000 | 2.641554 | 3785.7 | 4.090 | 3.485 | 9.084 | 12.807 | 0/10000 |
| 11 | 1 | 262144 | 16 | mem | true | 10000 | 0.867510 | 11527.2 | 1.246 | 1.084 | 2.678 | 3.594 | 0/10000 |
| 12 | 1 | 262144 | 16 | disk | true | 10000 | 0.678708 | 14733.9 | 0.944 | 0.833 | 1.980 | 2.592 | 0/10000 |
| 13 | 2 | 16384 | 16 | disk | true | 10000 | 0.219978 | 45459.1 | 0.340 | 0.285 | 0.761 | 1.105 | 0/10000 |
| 14 | 2 | 16384 | 16 | fresh | true | 10000 | 0.433485 | 23068.8 | 0.681 | 0.579 | 1.494 | 2.045 | 0/10000 |
| 15 | 2 | 16384 | 16 | mem | true | 10000 | 0.128942 | 77554.1 | 0.196 | 0.165 | 0.430 | 0.644 | 0/10000 |
| 16 | 2 | 262144 | 1 | fresh | true | 10000 | 9.105694 | 1098.2 | 0.806 | 0.799 | 0.903 | 0.979 | 0/10000 |
| 17 | 2 | 262144 | 1 | mem | true | 10000 | 2.585652 | 3867.5 | 0.158 | 0.155 | 0.175 | 0.213 | 0/10000 |
| 18 | 2 | 262144 | 1 | disk | true | 10000 | 2.947060 | 3393.2 | 0.193 | 0.190 | 0.212 | 0.266 | 0/10000 |
| 19 | 2 | 262144 | 16 | mem | true | 10000 | 0.864132 | 11572.3 | 1.233 | 1.052 | 2.709 | 3.694 | 0/10000 |
| 20 | 2 | 262144 | 16 | disk | true | 10000 | 0.677622 | 14757.5 | 0.939 | 0.823 | 1.991 | 2.560 | 0/10000 |
| 21 | 2 | 262144 | 16 | fresh | true | 10000 | 2.727875 | 3665.9 | 4.226 | 3.600 | 9.373 | 13.436 | 0/10000 |
| 22 | 2 | 16384 | 1 | mem | true | 10000 | 0.483356 | 20688.7 | 0.041 | 0.040 | 0.048 | 0.069 | 0/10000 |
| 23 | 2 | 16384 | 1 | disk | true | 10000 | 0.738000 | 13550.1 | 0.066 | 0.064 | 0.083 | 0.115 | 0/10000 |
| 24 | 2 | 16384 | 1 | fresh | true | 10000 | 1.330847 | 7514.0 | 0.125 | 0.116 | 0.193 | 0.268 | 0/10000 |
| 25 | 3 | 262144 | 1 | mem | true | 10000 | 2.570930 | 3889.6 | 0.156 | 0.155 | 0.170 | 0.199 | 0/10000 |
| 26 | 3 | 262144 | 1 | disk | true | 10000 | 2.956968 | 3381.8 | 0.194 | 0.190 | 0.213 | 0.268 | 0/10000 |
| 27 | 3 | 262144 | 1 | fresh | true | 10000 | 9.314481 | 1073.6 | 0.827 | 0.809 | 0.965 | 1.058 | 0/10000 |
| 28 | 3 | 262144 | 16 | disk | true | 10000 | 0.881985 | 11338.1 | 1.258 | 0.964 | 2.552 | 3.744 | 0/10000 |
| 29 | 3 | 262144 | 16 | fresh | true | 10000 | 2.670308 | 3744.9 | 4.135 | 3.542 | 9.108 | 12.876 | 0/10000 |
| 30 | 3 | 262144 | 16 | mem | true | 10000 | 0.874006 | 11441.6 | 1.251 | 1.085 | 2.688 | 3.569 | 0/10000 |
| 31 | 3 | 16384 | 1 | disk | true | 10000 | 0.740946 | 13496.3 | 0.066 | 0.064 | 0.085 | 0.121 | 0/10000 |
| 32 | 3 | 16384 | 1 | fresh | true | 10000 | 1.328101 | 7529.5 | 0.125 | 0.116 | 0.193 | 0.264 | 0/10000 |
| 33 | 3 | 16384 | 1 | mem | true | 10000 | 0.483806 | 20669.5 | 0.041 | 0.040 | 0.047 | 0.068 | 0/10000 |
| 34 | 3 | 16384 | 16 | fresh | true | 10000 | 0.432941 | 23097.8 | 0.680 | 0.573 | 1.532 | 2.050 | 0/10000 |
| 35 | 3 | 16384 | 16 | mem | true | 10000 | 0.126883 | 78812.6 | 0.193 | 0.165 | 0.418 | 0.595 | 0/10000 |
| 36 | 3 | 16384 | 16 | disk | true | 10000 | 0.211411 | 47301.2 | 0.326 | 0.272 | 0.746 | 1.086 | 0/10000 |

## Method

- Real loopback TCP HTTP/1.1 with keep-alive; no proxy, compression, conditional requests, application-level retries, browser cache, or load generator process per backend. Go Transport may retry an idempotent request after a reused connection failure; connection acquisitions are recorded.
- Closed-loop workers issue one request at a time. Latency starts before client.Do and ends after reading and closing the full response, before byte/hash validation. There is no arrival-rate scheduling or coordinated-omission correction.
- Every warmup and measured response must have HTTP 200, exact Content-Length, text/plain content type, and the exact expected body bytes and SHA-256. Cached routes must retain warmup ETag/cache timestamps; fresh routes must have no internal cache metadata.
- Throughput is verified successful requests divided by measured batch wall time. Wall time includes client scheduling, body reads, byte comparisons and SHA-256 verification; only latency samples exclude payload verification.
- Raw latency_ns contains one successful response sample per request in worker reservation order. p50/p95/p99 use nearest rank: sorted[ceil(p*N)-1]. Average is the arithmetic mean; no outliers are dropped.
- Each trial gets a fresh client connection pool and excluded warmup. Backend order rotates by repeat and case index; case order also rotates across repeats. The server remains alive for the complete run.
- Memory and disk application caches are warm. Disk files and filesystem pages are warmed too; this is not cold physical-disk I/O and operating-system page-cache memory is not measured.
- The server and verifying client share one machine. Client CPU, local network stack, scheduler and other processes can limit results. These are local comparison measurements, not production capacity claims.
- No process RSS, heap profile or total system memory measurement is made. Disk file-body bytes are inventory only, not proof of process-memory savings.
- Outside timing, disk file counts, sizes and SHA-256 hashes are checked after each trial against only the disk variants warmed so far. This verifies the backend labels against actual disk storage.

Before shutdown: 2 disk body files, 278528 body bytes. Clean shutdown: true; remaining runtime namespaces: 0. This inventory does not measure RSS, heap or filesystem page-cache memory.

Fixture SHA-256: `b1f105a82421a08725b5b9e1edf58f2d0972f4adf2ce0391a010b1d3dc674d45`. Runner source SHA-256: `787dd62213b56cf40894702adc5794cc7ccc20e34c5e05dde14610bca8736380`.

`result.json` contains every successful latency sample in nanoseconds, exact response hashes, connection counts, failures and provenance. `server.log` contains the isolated server output. Build and server startup are excluded from HTTP timing. Disk hits are warm filesystem/page-cache reads; these local results do not establish production capacity.
