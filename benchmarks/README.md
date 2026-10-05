# HyperBricks performance baselines

This directory contains reproducible measurements of HyperBricks and tools for
comparing its own versions. Each workload has an executable fixture, a documented
measurement method and immutable recorded results. The purpose is to show how
the same workload changes across HyperBricks revisions on the same machine.

| Workload | Fixture version | Recorded reference | Status |
| --- | --- | --- | --- |
| [Response-cache HTTP](response-cache/README.md) | [`response-cache-test` 1.0.0](../modules/response-cache-test/README.md) | [2026-10-05 baseline](response-cache/baselines/20261005/README.md) | Development baseline; dirty revision `4c6e9256b27b19e6244655d632240418ef00aee4`; no tagged-release measurement |

The response-cache fixture measures fresh rendering, memory-cache hits and
disk-cache hits with identical 16 KiB and 256 KiB responses. Its first record
establishes the baseline at this feature's introduction. It does not demonstrate
improvement over a release that lacked the feature. Earlier releases can only
be compared with workloads and configuration they actually support.

Exploratory experiments live separately under the Git-ignored
`internal-benchmarks/` directory. They are not part of the public baseline set.

## Compare a saved run with a current run

Use Python 3.9 or newer; the comparator has no external dependencies:

```sh
python3 benchmarks/compare-response-cache.py /path/to/prior/result.json /path/to/current/result.json
```

The first argument is the reference and the second is the candidate. Output is
Markdown on stdout; use `--json` for structured output. The command reads the
original files without changing them. A valid comparison exits with status 0;
invalid, incomplete or incompatible input exits with status 2 and explains the
reason. Performance changes themselves do not change the exit status.

**Recommended:** build one frozen runner and measure both server binaries with
it, using the paired workflow in the next section. The comparator requires an
identical executed runner binary as well as matching source manifests.

If you retained that exact runner binary from a previously saved run with the
published baseline settings, reuse it to measure the current server **on the
same physical machine**, with the same OS, Go toolchain and tuning and other
builds/load generators stopped:

```sh
# Set this to the actual earlier run made on this same machine.
prior_result=/absolute/path/to/prior/result.json
frozen_runner=/absolute/path/to/the/retained/runner
comparison_dir=$(mktemp -d "${TMPDIR:-/tmp}/hyperbricks-comparison.XXXXXX")
go build -trimpath -o "$comparison_dir/current-hyperbricks" ./cmd/hyperbricks

"$frozen_runner" -repo "$PWD" -binary "$comparison_dir/current-hyperbricks" \
  -requests 10000 -warmup 1000 -repeats 3 \
  -sizes 16384,262144 -concurrency 1,16 -client-procs 4 \
  -output "$comparison_dir/current"

python3 benchmarks/compare-response-cache.py \
  "$prior_result" "$comparison_dir/current/result.json" \
  > "$comparison_dir/comparison.md"
```

The archived 2026-10-05 result can be the reference only when its recorded
environment, source manifests and executed runner binary match and it came
from the same machine. The archive preserves the runner's hash, not its
executable. It remains historical development evidence; arbitrary future runs
are not automatically comparable with it. Matching CPU model and OS strings do
not prove that two runs used the same physical host. On another machine, or
without the identical runner, make a new pair of measurements.

## Measure two server versions with one fixed runner and fixture

Keep an existing checkout of the earlier server revision outside the current
checkout. It must support this fixture's response-cache configuration. Build
both server binaries and one runner before either measurement, using the same
Go toolchain and build flags:

```sh
# Set this to an existing checkout of the earlier supported revision.
prior_checkout=/absolute/path/to/earlier-hyperbricks-checkout
comparison_dir=$(mktemp -d "${TMPDIR:-/tmp}/hyperbricks-two-versions.XXXXXX")
runner_checkout=$PWD

go build -trimpath -o "$comparison_dir/runner" ./benchmarks/response-cache
go build -trimpath -o "$comparison_dir/current-hyperbricks" ./cmd/hyperbricks
(
  cd "$prior_checkout"
  go build -trimpath -o "$comparison_dir/prior-hyperbricks" ./cmd/hyperbricks
)

"$comparison_dir/runner" -repo "$runner_checkout" \
  -binary "$comparison_dir/prior-hyperbricks" -output "$comparison_dir/prior" \
  -requests 10000 -warmup 1000 -repeats 3 \
  -sizes 16384,262144 -concurrency 1,16 -client-procs 4

"$comparison_dir/runner" -repo "$runner_checkout" \
  -binary "$comparison_dir/current-hyperbricks" -output "$comparison_dir/current" \
  -requests 10000 -warmup 1000 -repeats 3 \
  -sizes 16384,262144 -concurrency 1,16 -client-procs 4

python3 benchmarks/compare-response-cache.py \
  "$comparison_dir/prior/result.json" "$comparison_dir/current/result.json" \
  > "$comparison_dir/comparison.md"
```

The fixed runner stages fresh temporary copies of its fixture for each run.
Both runs use the same live-mode settings and four server Go execution slots.
The source checkout and its module cache are not modified by the measurements.
Review repeat ranges and rerun with the version order reversed when a change
is small or variation is substantial.

The **server identity** comes from the measured binary's embedded
`vcs.revision`/`vcs.modified` build settings and its SHA-256. If the binary has no
embedded revision, the comparator says so and retains the binary hash. The
top-level `provenance.git_revision` identifies the **runner checkout**; when
testing two prebuilt servers it can legitimately be identical in both records.
It must not be presented as the earlier server's revision.

## What the comparison verifies

The comparator requires complete valid runs with clean server shutdown, the
full repeated trial matrix, successful response checks, exact expected payload
hashes and the corresponding disk inventory. It recalculates trial throughput
and latency statistics from counts, elapsed times and raw samples, checks them
against the recorded trial statistics, then calculates comparison medians.
Saved aggregate summaries are not used as inputs.

Comparisons require matching:

- The executed runner binary SHA-256, so both versions used the same client.
- Fixture and runner source fingerprints, including internally consistent
  source-file manifests; the staged fixture must match its recorded source.
- Body sizes, backends, concurrency, request/warmup counts, repeats and actual
  trial order, request deadlines, execution slots and measurement methodology.
- Recorded CPU model, logical CPU count, architecture, OS/kernel versions, Go
  versions, compiler/build settings and inherited Go runtime tuning.

Server revision, dirty state and binary hash may differ, as expected for version
comparisons. Checkout/output paths, temporary ports and observed startup times
do not need to match. Compiler settings are compared without the `vcs.*`
metadata. There is no override for incompatible inputs: keep them as separate
measurement series or rerun under matching conditions.

If runner binary hashes differ, rerun both server binaries with one frozen
runner. Rebuilding unchanged runner code can produce another hash because Go
embeds VCS metadata. The source manifest is read from the checkout and cannot
prove that an independently supplied runner binary was built from it; matching
manifests alone are insufficient. Server binary hashes may differ.

The report shows median throughput and its observed trial range, plus median
trial average/p50/p95/p99 latency before and after. Percentage changes are
`(after / before - 1) × 100`: positive throughput is higher; negative latency is
lower. Percentiles are not pooled or averaged across trials. There is no
automatic regression gate, significance threshold or claim that a small delta
exceeds measurement noise.

The recorded environment cannot establish equal background load, thermal state,
power configuration or physical-host identity. Preserve that context when
publishing a comparison. The verifying client shares CPU with the server and
includes body/hash validation in throughput. Disk cache results use warm
filesystem pages. These results do not measure cold-disk I/O, process RSS or
production request capacity.

## Versioning and publication

`response-cache-test` currently declares module version `1.0.0`; the baseline's
exact fixture SHA-256 is
`b1f105a82421a08725b5b9e1edf58f2d0972f4adf2ce0391a010b1d3dc674d45`.
The version communicates the intended workload contract. The per-file manifest
and aggregate fingerprint identify exactly what was measured. Full fixture and
runner manifests include their documentation, so even a documentation change
changes the fingerprint; preserve the recorded sources for a direct comparison.

When workload behavior or the measurement method changes, version the fixture
or runner as appropriate and establish a new baseline series. Compare like
with like. A new feature starts its performance history when a supported
executable workload first exists.

Keep original JSON, raw trials, report and relevant logs unchanged under a
dated workload `baselines/` directory. Record whether a run used a clean tag,
a commit or a dirty development checkout. Only a run actually measured from
the identified clean release source should be described as a release baseline;
a later tag does not retroactively turn the 2026-10-05 development record into
a tagged-release measurement. Ordinary local `results/` output stays ignored.

Comparator verification, without starting a server or a benchmark:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s benchmarks -p test_compare_response_cache.py
python3 benchmarks/compare-response-cache.py \
  benchmarks/response-cache/baselines/20261005/result.json \
  benchmarks/response-cache/baselines/20261005/result.json
```

The self-comparison should report zero deltas for all twelve cases. It verifies
the comparison path; it is not an additional performance measurement.
