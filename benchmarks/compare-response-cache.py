#!/usr/bin/env python3
"""Compare two completed response-cache runs without a performance pass/fail gate."""

import argparse
from collections import defaultdict
from datetime import datetime
import hashlib
import json
import math
from pathlib import Path
import re
import statistics
import sys


class InvalidRun(ValueError):
    """Missing evidence, invalid measurement, or incompatible comparison inputs."""


def require(condition, message):
    if not condition:
        raise InvalidRun(message)


def positive_int(value):
    return type(value) is int and value > 0


def positive_number(value):
    return type(value) in (int, float) and math.isfinite(value) and value > 0


def same_number(actual, expected, name):
    require(positive_number(actual) and math.isclose(actual, expected, rel_tol=1e-12),
            f"{name} disagrees with raw measurements")


def digest(value):
    return isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value) is not None


def parse_timestamp(value):
    """Read Go's RFC3339Nano output on Python 3.9 without losing nanoseconds."""
    require(isinstance(value, str), "timestamp must be an RFC3339 string")
    match = re.fullmatch(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})", value)
    require(match is not None, "timestamp must include an RFC3339 timezone and at most nine fractional digits")
    # Python 3.9 accepts only some fractional widths. Parse whole seconds with
    # the standard library, then compare the independent nanosecond remainder.
    seconds = datetime.fromisoformat(match[1] + ("+00:00" if match[3] == "Z" else match[3]))
    return seconds, int((match[2] or "0").ljust(9, "0"))


def fingerprint(manifest, name):
    require(isinstance(manifest, dict) and digest(manifest.get("sha256")), f"{name}: missing SHA-256 fingerprint")
    files = manifest.get("files")
    require(isinstance(files, list) and files, f"{name}: missing source-file manifest")
    paths = []
    normalized = []
    for item in files:
        require(isinstance(item, dict), f"{name}: malformed source entry")
        path = item.get("path")
        require(isinstance(path, str) and path and digest(item.get("sha256")), f"{name}: malformed path/hash")
        require(type(item.get("bytes")) is int and item["bytes"] >= 0 and positive_int(item.get("mode")), f"{name}: malformed size/mode")
        paths.append(path)
        normalized.append({key: item[key] for key in ("path", "bytes", "sha256", "mode")})
    require(paths == sorted(set(paths)), f"{name}: source paths must be unique and sorted")
    # Match the Go runner's compact JSON encoding, including HTML escaping.
    encoded = json.dumps(normalized, ensure_ascii=False, separators=(",", ":"))
    for char, escape in (("<", "\\u003c"), (">", "\\u003e"), ("&", "\\u0026"), ("\u2028", "\\u2028"), ("\u2029", "\\u2029")):
        encoded = encoded.replace(char, escape)
    require(hashlib.sha256(encoded.encode()).hexdigest() == manifest["sha256"], f"{name}: fingerprint disagrees with file manifest")
    return manifest["sha256"]


def body_hash(size):
    result = hashlib.sha256()
    block = b"0123456789abcdef" * 4096
    for _ in range(size // len(block)):
        result.update(block)
    result.update(block[:size % len(block)])
    return result.hexdigest()


def validate_run(run):
    """Validate format v1 evidence and return its comparable contract + summaries."""
    require(isinstance(run, dict) and run.get("format_version") == 1, "unsupported or missing format_version (expected 1)")
    require(run.get("valid") is True and not run.get("error"), "run is invalid or records an error")
    try:
        started = parse_timestamp(run["started_at_utc"])
        completed = parse_timestamp(run["completed_at_utc"])
        require(completed > started, "run has no valid completion interval")
        options, environment, provenance, server = (run[key] for key in ("options", "environment", "provenance", "server"))
        option_names = ("requests_per_trial", "warmup_requests_per_trial", "repeats", "client_gomaxprocs", "server_gomaxprocs", "request_timeout_ns", "startup_timeout_ns")
        contract = {f"options.{key}": options[key] for key in option_names}
        require(all(positive_int(value) for value in contract.values()), "counts, execution slots and timeouts must be positive integers")
        sizes, workers = options["body_sizes_bytes"], options["concurrency"]
        for label, values in (("body_sizes_bytes", sizes), ("concurrency", workers)):
            require(isinstance(values, list) and values and all(positive_int(v) for v in values) and len(set(values)) == len(values), f"options.{label} must contain distinct positive integers")
            contract[f"options.{label}"] = values
        requests, repeats = options["requests_per_trial"], options["repeats"]
        require(all(size % 16 == 0 for size in sizes), "body sizes do not match the fixture's 16-byte pattern")
        require(max(workers) <= min(requests, options["warmup_requests_per_trial"]), "worker count exceeds measured or warmup requests")
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
        require(isinstance(provenance["git_revision"], str) and re.fullmatch(r"[0-9a-f]{40,64}", provenance["git_revision"]), "missing Git revision")
        require(type(provenance["git_dirty"]) is bool, "missing Git dirty status")
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
        method = run["methodology"]
        require(isinstance(method, list) and method and all(isinstance(item, str) and item for item in method), "missing measurement methodology")
        contract["methodology"] = method
        require(server["shutdown_clean"] is True and server["namespaces_after_shutdown"] == 0, "server did not shut down and clean its cache")
        require(server["disk_entries_before_shutdown"] == len(sizes) and server["disk_body_bytes_before_shutdown"] == sum(sizes), "disk inventory does not match the payload matrix")
        trials = run["trials"]
        require(isinstance(trials, list) and len(trials) == len(sizes)*len(workers)*3*repeats, "incomplete trial matrix")
        groups, seen, order = defaultdict(list), set(), []
        hashes = {size: body_hash(size) for size in sizes}
        for sequence, trial in enumerate(trials, 1):
            label = f"trial {sequence}"
            require(trial["sequence"] == sequence and trial["valid"] is True and not trial.get("error"), f"{label}: invalid sequence or result")
            key = (trial["body_bytes"], trial["concurrency"], trial["backend"])
            repeat = trial["repeat"]
            require(key[0] in sizes and key[1] in workers and key[2] in ("fresh", "mem", "disk") and type(repeat) is int and 1 <= repeat <= repeats, f"{label}: unknown matrix member")
            require((*key, repeat) not in seen, f"{label}: duplicated matrix member")
            seen.add((*key, repeat))
            order.append((*key, repeat))
            require(trial["requested"] == requests and trial["verified"] == requests and trial["warmup_requests"] == options["warmup_requests_per_trial"], f"{label}: incomplete measured/warmup requests")
            require(trial["failure_count"] == 0 and not trial.get("failures"), f"{label}: request failures recorded")
            connections = [trial[name] for name in ("new_connections", "reused_connections")]
            require(all(type(value) is int and value >= 0 for value in connections) and sum(connections) >= requests, f"{label}: connection evidence incomplete")
            require(positive_int(trial["elapsed_ns"]) and positive_int(trial["warmup_elapsed_ns"]), f"{label}: missing elapsed measurement")
            checks = trial["checks"]
            require(all(checks[key] is True for key in ("every_body_compared", "every_body_sha256_checked", "render_error_count_zero_checked")), f"{label}: response checks incomplete")
            require(checks["expected_status"] == 200 and checks["expected_media_type"] == "text/plain" and checks["verified_measured_body_bytes"] == requests*key[0] and checks["expected_body_sha256"] == hashes[key[0]], f"{label}: body/status verification does not match fixture")
            require(checks["cached_metadata_stable"] is (key[2] != "fresh"), f"{label}: wrong cache policy verification")
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
            same_number(trial["body_mib_per_second"], stats["requests_per_second"]*key[0]/1048576, f"{label} byte throughput")
            groups[key].append(stats)
        contract["trial_order"] = order
        summaries = {}
        for key, values in groups.items():
            summary = {name: statistics.median(v[name] for v in values) for name in ("requests_per_second", "average_ns", "p50_ns", "p95_ns", "p99_ns")}
            rates = [v["requests_per_second"] for v in values]
            summary.update(min_requests_per_second=min(rates), max_requests_per_second=max(rates), trials=len(values))
            summaries[key] = summary
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
    for key in sorted(summaries[0], key=lambda value: (*value[:2], ("fresh", "mem", "disk").index(value[2]))):
        old, new = summaries[0][key], summaries[1][key]
        rows.append({"body_bytes": key[0], "concurrency": key[1], "backend": key[2], "before": old, "after": new,
                     "change_percent": {name: (new[name]/old[name]-1)*100 for name in ("requests_per_second", "average_ns", "p50_ns", "p95_ns", "p99_ns")}})
    return {"comparable": True, "before": identity(before), "after": identity(after), "rows": rows}


def identity(run):
    provenance = run["provenance"]
    binary = provenance["server_binary"]
    settings = binary["build_settings"]
    return {"server_revision": settings.get("vcs.revision"), "server_modified": settings.get("vcs.modified"),
            "server_binary_sha256": binary["sha256"], "runner_checkout_revision": provenance["git_revision"],
            "runner_checkout_dirty": provenance["git_dirty"], "runner_binary_sha256": provenance["runner_binary"]["sha256"]}


def markdown(result, before_path, after_path):
    lines = ["# HyperBricks response-cache comparison", "", f"Before: `{before_path}`; after: `{after_path}`.", ""]
    for label in ("before", "after"):
        record = result[label]
        lines.append(f"{label.capitalize()} server revision: `{record['server_revision'] or 'not embedded'}`, modified={record['server_modified'] or 'unknown'}; binary SHA-256: `{record['server_binary_sha256']}`.")
        lines.append(f"{label.capitalize()} runner checkout: `{record['runner_checkout_revision']}`, dirty={str(record['runner_checkout_dirty']).lower()}.")
        lines.append(f"{label.capitalize()} runner binary SHA-256: `{record['runner_binary_sha256']}`.")
    lines += ["", "Recorded workload, fixture, runner source and executed runner binary, hardware, OS, Go versions, build flags and execution settings match. Hardware metadata cannot prove the same physical host or equivalent background load; confirm both runs used the same machine.", "",
              "Changes are (after / before − 1) × 100. Positive throughput is higher; negative latency is lower. Latencies are medians of individual trial statistics. No regression gate or statistical-significance claim is applied.", "",
              "| Bytes | Workers | Backend | Median req/s before → after | Change | Observed req/s range before → after | Avg ms before → after (change) | p50 ms before → after (change) | p95 ms before → after (change) | p99 ms before → after (change) |",
              "| ---: | ---: | --- | ---: | ---: | --- | --- | --- | --- | --- |"]
    for row in result["rows"]:
        old, new, delta = row["before"], row["after"], row["change_percent"]
        latency = [f"{old[name]/1e6:.3f} → {new[name]/1e6:.3f} ({delta[name]:+.2f}%)" for name in ("average_ns", "p50_ns", "p95_ns", "p99_ns")]
        lines.append(f"| {row['body_bytes']} | {row['concurrency']} | {row['backend']} | {old['requests_per_second']:.1f} → {new['requests_per_second']:.1f} | {delta['requests_per_second']:+.2f}% | {old['min_requests_per_second']:.1f}–{old['max_requests_per_second']:.1f} → {new['min_requests_per_second']:.1f}–{new['max_requests_per_second']:.1f} | " + " | ".join(latency) + " |")
    lines += ["", "The comparator recalculates trial statistics and medians from raw samples/counts, rather than trusting saved aggregate summaries. Observed ranges are descriptive, not confidence intervals. Review repeat variation and rerun before attributing small changes to the server.", "",
              "These are warm-cache loopback measurements with a verifying client sharing server CPU. They do not measure cold disk I/O, RSS, or production capacity."]
    return "\n".join(lines) + "\n"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("before", type=Path, help="earlier response-cache result.json")
    parser.add_argument("after", type=Path, help="later response-cache result.json")
    parser.add_argument("--json", action="store_true", help="emit structured comparison instead of Markdown")
    args = parser.parse_args(argv)
    try:
        runs = [json.loads(path.read_text()) for path in (args.before, args.after)]
        result = compare(*runs)
    except (OSError, ValueError) as error:
        print(f"Comparison refused: {error}", file=sys.stderr)
        return 2
    print(json.dumps(result, indent=2, allow_nan=False) if args.json else markdown(result, args.before, args.after), end="\n" if args.json else "")
    return 0


if __name__ == "__main__":
    sys.exit(main())
