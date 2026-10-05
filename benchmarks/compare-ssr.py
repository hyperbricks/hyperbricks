#!/usr/bin/env python3
"""Compare two verified HyperBricks SSR runs without a performance pass/fail gate."""

import argparse
from collections import defaultdict
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import statistics
import sys


# Reuse the fingerprint, numeric, timestamp and binary identity checks shared
# by the HTTP benchmark comparators.
SPEC = importlib.util.spec_from_file_location("response_cache_comparison", Path(__file__).with_name("compare-response-cache.py"))
COMMON = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(COMMON)
InvalidRun = COMMON.InvalidRun
require = COMMON.require
positive_int = COMMON.positive_int
same_number = COMMON.same_number
digest = COMMON.digest
fingerprint = COMMON.fingerprint


def environment_contract(options, environment, provenance):
    """Capture the actual host/client contract separately from engine identity."""
    contract = {}
    for key in ("os", "architecture", "os_version", "kernel", "cpu", "client_go_version"):
        require(isinstance(environment[key], str) and environment[key] and environment[key] != "unavailable", f"environment.{key} is unavailable")
        contract[f"environment.{key}"] = environment[key]
    for key in ("logical_cpus", "client_gomaxprocs"):
        require(positive_int(environment[key]), f"environment.{key} must be positive")
        contract[f"environment.{key}"] = environment[key]
    require(environment["client_gomaxprocs"] == options["client_gomaxprocs"], "client execution slots disagree")
    tuning = environment["inherited_go_tuning"]
    require(isinstance(tuning, dict) and all(isinstance(k, str) and isinstance(v, str) for k, v in tuning.items()), "malformed Go environment tuning")
    contract["environment.inherited_go_tuning"] = tuning
    require(isinstance(provenance["git_revision"], str) and re.fullmatch(r"[0-9a-f]{40,64}", provenance["git_revision"]), "missing runner checkout revision")
    require(type(provenance["git_dirty"]) is bool, "missing runner checkout dirty status")
    for key in ("server_binary", "runner_binary"):
        binary = provenance[key]
        require(digest(binary["sha256"]) and positive_int(binary["bytes"]), f"{key}: missing binary identity")
        require(isinstance(binary["go_version"], str) and binary["go_version"], f"{key}: missing Go version")
        contract[f"{key}.go_version"] = binary["go_version"]
        settings = binary["build_settings"]
        require(isinstance(settings, dict) and settings and all(isinstance(k, str) and isinstance(v, str) for k, v in settings.items()), f"{key}: missing build settings")
        require(settings.get("GOOS") == environment["os"] and settings.get("GOARCH") == environment["architecture"], f"{key}: build platform disagrees with host")
        contract[f"{key}.build_settings"] = {k: v for k, v in settings.items() if k != "vcs" and not k.startswith("vcs.")}
    require(provenance["runner_binary"]["go_version"] == environment["client_go_version"], "client Go versions disagree")
    contract["runner_binary.sha256"] = provenance["runner_binary"]["sha256"]
    fixture = fingerprint(provenance["fixture_source"], "fixture_source")
    require(fingerprint(provenance["isolated_fixture"], "isolated_fixture") == fixture, "staged fixture differs from source")
    contract["fixture_source.sha256"] = fixture
    contract["runner_source.sha256"] = fingerprint(provenance["runner_source"], "runner_source")
    return contract


def validate_run(run):
    """Validate SSR format v1 and recalculate all statistics used in comparison."""
    require(isinstance(run, dict) and type(run.get("format_version")) is int and run["format_version"] == 1,
            "unsupported or missing format_version (expected 1)")
    require(run.get("benchmark") == "ssr", "expected an SSR benchmark result")
    require(run.get("valid") is True and not run.get("error"), "run is invalid or records an error")
    try:
        started = COMMON.parse_timestamp(run["started_at_utc"])
        completed = COMMON.parse_timestamp(run["completed_at_utc"])
        require(completed > started, "run has no valid completion interval")
        options, environment, provenance, server = (run[key] for key in ("options", "environment", "provenance", "server"))
        names = ("requests_per_trial", "warmup_requests_per_trial", "repeats", "client_gomaxprocs", "server_gomaxprocs", "request_timeout_ns", "startup_timeout_ns")
        contract = {f"options.{key}": options[key] for key in names}
        require(all(positive_int(value) for value in contract.values()), "counts, execution slots and timeouts must be positive integers")
        require(options["profile"] == "package.hyperbricks.yaml" and options["log_level"] == "info", "unsupported SSR profile or logging contract")
        contract.update({f"options.{key}": options[key] for key in ("profile", "log_level")})
        workers = options["concurrency"]
        require(isinstance(workers, list) and workers and all(positive_int(v) for v in workers) and len(set(workers)) == len(workers), "concurrency must contain distinct positive integers")
        requests, repeats = options["requests_per_trial"], options["repeats"]
        require(max(workers) <= min(requests, options["warmup_requests_per_trial"]), "worker count exceeds measured or warmup requests")
        contract["options.concurrency"] = workers
        contract.update(environment_contract(options, environment, provenance))
        method = run["methodology"]
        require(isinstance(method, list) and method and all(isinstance(item, str) and item for item in method), "missing measurement methodology")
        contract["methodology"] = method
        workload = run["workload"]
        require(isinstance(workload, dict) and digest(workload["canonical_template_sha256"]), "missing canonical SSR workload hash")
        require(workload["name"] == "nested-request-isolated-html" and workload["version"] == 1 and workload["profile"] == "package.hyperbricks.yaml", "unsupported SSR workload")
        template = workload["canonical_template"]
        require(isinstance(template, str) and template.count("__REQUEST_ID__") == 3, "canonical template must contain three request-ID markers")
        require(hashlib.sha256(template.encode()).hexdigest() == workload["canonical_template_sha256"], "canonical template fingerprint disagrees with recorded template")
        require(workload["request_id_format"] == "ssr-<16lowerhex>-<4digittrial>-<w|m>-<10digitindex>" and workload["request_id_bytes"] == 38 and workload["request_id_occurrences"] == 3, "unsupported request-ID format or isolation contract")
        require(positive_int(workload["body_bytes"]), "missing SSR response size")
        require(workload["body_bytes"] == len(template.replace("__REQUEST_ID__", "x"*38).encode()), "SSR response size disagrees with canonical template")
        contract["workload"] = workload
        require(server["shutdown_clean"] is True, "server did not shut down cleanly")
        require(server["response_cache_entries_after_run"] == 0, "SSR unexpectedly stored response bodies")
        preflight = run["preflight"]
        require(preflight["valid"] is True and not preflight.get("error") and preflight["requests"] == 4,
                "escaping/request-isolation preflight is incomplete or failed")
        trials = run["trials"]
        require(isinstance(trials, list) and len(trials) == len(workers)*repeats, "incomplete SSR trial matrix")
        groups, seen, prefixes, order = defaultdict(list), set(), set(), []
        for sequence, trial in enumerate(trials, 1):
            label = f"trial {sequence}"
            require(trial["sequence"] == sequence and trial["valid"] is True and not trial.get("error"), f"{label}: invalid sequence or result")
            concurrency, repeat = trial["concurrency"], trial["repeat"]
            require(type(concurrency) is int and concurrency in workers and type(repeat) is int and 1 <= repeat <= repeats, f"{label}: unknown matrix member")
            require((concurrency, repeat) not in seen, f"{label}: duplicated matrix member")
            seen.add((concurrency, repeat))
            order.append((concurrency, repeat))
            require(trial["body_bytes"] == workload["body_bytes"], f"{label}: inconsistent response size")
            prefix = trial["measured_request_id_prefix"]
            require(isinstance(prefix, str) and re.fullmatch(rf"ssr-[0-9a-f]{{16}}-{sequence:04d}-m-", prefix) and prefix not in prefixes, f"{label}: missing, unsafe or repeated request-ID prefix")
            require(trial["warmup_request_id_prefix"] == prefix[:-2] + "w-", f"{label}: warmup and measured IDs are not separated")
            prefixes.add(prefix)
            require(trial["requested"] == requests and trial["verified"] == requests and trial["warmup_requests"] == options["warmup_requests_per_trial"], f"{label}: incomplete measured/warmup requests")
            require(trial["failure_count"] == 0 and not trial.get("failures"), f"{label}: request failures recorded")
            connections = [trial[name] for name in ("new_connections", "reused_connections")]
            require(all(type(value) is int and value >= 0 for value in connections) and sum(connections) >= requests, f"{label}: connection evidence incomplete")
            require(positive_int(trial["elapsed_ns"]) and positive_int(trial["warmup_elapsed_ns"]), f"{label}: missing elapsed measurement")
            checks = trial["checks"]
            require(all(checks[key] is True for key in ("every_body_compared", "every_body_sha256_checked", "unique_request_ids", "cache_metadata_absent", "render_error_count_zero_checked", "request_id_header_present")), f"{label}: response or request-isolation checks incomplete")
            require(checks["request_id_occurrences"] == 3 and checks["cache_control"] == "no-store", f"{label}: wrong request-isolation or no-cache contract")
            require(checks["expected_status"] == 200 and checks["expected_media_type"] == "text/html" and checks["verified_measured_body_bytes"] == requests*workload["body_bytes"], f"{label}: response verification does not match SSR workload")
            samples = trial["latency_ns"]
            require(isinstance(samples, list) and len(samples) == requests and all(positive_int(sample) for sample in samples), f"{label}: raw latency samples incomplete or invalid")
            ordered = sorted(samples)
            stats = {"average_ns": sum(samples)/requests, "min_ns": ordered[0], "max_ns": ordered[-1]}
            stats.update({f"p{p}_ns": ordered[(p*requests+99)//100-1] for p in (50, 95, 99)})
            require(trial["latency"]["sample_count"] == requests, f"{label}: latency sample count mismatch")
            for name, value in stats.items():
                same_number(trial["latency"][name], value, f"{label} {name}")
            stats["requests_per_second"] = requests*1e9/trial["elapsed_ns"]
            same_number(trial["requests_per_second"], stats["requests_per_second"], f"{label} throughput")
            same_number(trial["body_mib_per_second"], stats["requests_per_second"]*workload["body_bytes"]/1048576, f"{label} byte throughput")
            groups[concurrency].append(stats)
        contract["trial_order"] = order
        summaries = {}
        for concurrency, values in groups.items():
            summary = {name: statistics.median(v[name] for v in values) for name in ("requests_per_second", "average_ns", "p50_ns", "p95_ns", "p99_ns")}
            rates = [v["requests_per_second"] for v in values]
            summary.update(min_requests_per_second=min(rates), max_requests_per_second=max(rates), trials=len(values))
            summaries[concurrency] = summary
        return contract, summaries
    except (KeyError, TypeError, AttributeError, OverflowError) as error:
        raise InvalidRun(f"missing or malformed evidence: {error}") from error
    except ValueError as error:
        if isinstance(error, InvalidRun):
            raise
        raise InvalidRun(f"malformed value: {error}") from error


def compare(before, after):
    contracts, summaries = [], []
    for label, run in (("before", before), ("after", after)):
        try:
            contract, summary = validate_run(run)
        except InvalidRun as error:
            raise InvalidRun(f"{label}: {error}") from error
        contracts.append(contract)
        summaries.append(summary)
    differences = [key for key in contracts[0] if contracts[0][key] != contracts[1][key]]
    require(not differences, "runs are not comparable; mismatched " + ", ".join(differences))
    rows = []
    for workers in sorted(summaries[0]):
        old, new = summaries[0][workers], summaries[1][workers]
        rows.append({"concurrency": workers, "body_bytes": before["workload"]["body_bytes"], "before": old, "after": new,
                     "change_percent": {name: (new[name]/old[name]-1)*100 for name in ("requests_per_second", "average_ns", "p50_ns", "p95_ns", "p99_ns")}})
    return {"comparable": True, "benchmark": "ssr", "before": COMMON.identity(before), "after": COMMON.identity(after), "rows": rows}


def markdown(result, before_path, after_path):
    lines = ["# HyperBricks SSR comparison", "", f"Before: `{before_path}`; after: `{after_path}`.", ""]
    for label in ("before", "after"):
        record = result[label]
        lines.append(f"{label.capitalize()} server revision: `{record['server_revision'] or 'not embedded'}`, modified={record['server_modified'] or 'unknown'}; binary SHA-256: `{record['server_binary_sha256']}`.")
        lines.append(f"{label.capitalize()} runner checkout: `{record['runner_checkout_revision']}`, dirty={str(record['runner_checkout_dirty']).lower()}; runner binary SHA-256: `{record['runner_binary_sha256']}`.")
    lines += ["", "Recorded SSR workload, fixture, runner source and executed runner binary, hardware, OS, Go versions, build flags and execution settings match. Matching machine metadata cannot establish physical-host identity or equal background load; confirm both measurements used the same machine.", "",
              "Changes are (after / before − 1) × 100. Positive throughput is higher; negative latency is lower. Latencies are medians of individual trial statistics. No regression gate or statistical-significance claim is applied.", "",
              "| Bytes | Workers | Median req/s before → after | Change | Observed req/s range before → after | Avg ms before → after (change) | p50 ms before → after (change) | p95 ms before → after (change) | p99 ms before → after (change) |",
              "| ---: | ---: | ---: | ---: | --- | --- | --- | --- | --- |"]
    for row in result["rows"]:
        old, new, delta = row["before"], row["after"], row["change_percent"]
        latency = [f"{old[name]/1e6:.3f} → {new[name]/1e6:.3f} ({delta[name]:+.2f}%)" for name in ("average_ns", "p50_ns", "p95_ns", "p99_ns")]
        lines.append(f"| {row['body_bytes']} | {row['concurrency']} | {old['requests_per_second']:.1f} → {new['requests_per_second']:.1f} | {delta['requests_per_second']:+.2f}% | {old['min_requests_per_second']:.1f}–{old['max_requests_per_second']:.1f} → {new['min_requests_per_second']:.1f}–{new['max_requests_per_second']:.1f} | " + " | ".join(latency) + " |")
    lines += ["", "Trial statistics and medians are recalculated from raw latency samples, counts and elapsed times. Observed ranges are descriptive, not confidence intervals. Request-specific HTML is verified in full; response caching is disabled. These loopback results include a verifying client sharing server CPU and do not establish production capacity or memory consumption."]
    return "\n".join(lines) + "\n"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("before", type=Path, help="earlier SSR result.json")
    parser.add_argument("after", type=Path, help="later SSR result.json")
    parser.add_argument("--json", action="store_true", help="emit structured comparison instead of Markdown")
    args = parser.parse_args(argv)
    try:
        result = compare(*(json.loads(path.read_text()) for path in (args.before, args.after)))
    except (OSError, ValueError) as error:
        print(f"Comparison refused: {error}", file=sys.stderr)
        return 2
    print(json.dumps(result, indent=2, allow_nan=False) if args.json else markdown(result, args.before, args.after), end="\n" if args.json else "")
    return 0


if __name__ == "__main__":
    sys.exit(main())
