"""SSR comparison safety tests; no server or timed workload is started."""

from copy import deepcopy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("compare-ssr.py")
SPEC = importlib.util.spec_from_file_location("compare_ssr", SCRIPT)
COMPARE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(COMPARE)


def valid_run():
    files = [{"path": "fixture.txt", "bytes": 16, "sha256": hashlib.sha256(b"0123456789abcdef").hexdigest(), "mode": 420}]
    manifest = {"files": files, "sha256": hashlib.sha256(json.dumps(files, separators=(",", ":")).encode()).hexdigest()}
    binary = {"sha256": "a"*64, "bytes": 100, "go_version": "go1.26.1", "build_settings": {"GOOS": "darwin", "GOARCH": "arm64", "-compiler": "gc", "CGO_ENABLED": "1", "vcs.revision": "1"*40, "vcs.modified": "false"}}
    template = '<main data-request-id="__REQUEST_ID__">Request __REQUEST_ID__<p>__REQUEST_ID__</p></main>'
    size = len(template.replace("__REQUEST_ID__", "x"*38))
    run = {
        "format_version": 1, "benchmark": "ssr", "valid": True,
        "started_at_utc": "2026-10-05T12:00:00Z", "completed_at_utc": "2026-10-05T12:01:00Z",
        "options": {"requests_per_trial": 4, "warmup_requests_per_trial": 4, "repeats": 3, "concurrency": [1, 4], "client_gomaxprocs": 4, "server_gomaxprocs": 4, "request_timeout_ns": 10000000000, "startup_timeout_ns": 30000000000, "profile": "package.hyperbricks.yaml", "log_level": "info"},
        "environment": {"os": "darwin", "architecture": "arm64", "os_version": "26.6.2", "kernel": "Darwin 25.6.0", "cpu": "Apple M3", "logical_cpus": 8, "client_go_version": "go1.26.1", "client_gomaxprocs": 4, "inherited_go_tuning": {}},
        "provenance": {"git_revision": "2"*40, "git_dirty": False, "server_binary": deepcopy(binary), "runner_binary": deepcopy(binary), "fixture_source": deepcopy(manifest), "isolated_fixture": deepcopy(manifest), "runner_source": deepcopy(manifest)},
        "workload": {"name": "nested-request-isolated-html", "version": 1, "profile": "package.hyperbricks.yaml", "canonical_template": template, "canonical_template_sha256": hashlib.sha256(template.encode()).hexdigest(), "request_id_format": "ssr-<16lowerhex>-<4digittrial>-<w|m>-<10digitindex>", "request_id_bytes": 38, "request_id_occurrences": 3, "body_bytes": size},
        "methodology": ["Test fixture: four fully verified SSR samples per trial."],
        "preflight": {"valid": True, "requests": 4},
        "server": {"shutdown_clean": True, "response_cache_entries_after_run": 0},
        "trials": [],
    }
    for repeat, elapsed in enumerate((4000000, 2000000, 8000000), 1):
        for concurrency in (1, 4):
            sequence = len(run["trials"])+1
            prefix = f"ssr-1234567890abcdef-{sequence:04d}-"
            run["trials"].append({"sequence": sequence, "repeat": repeat, "concurrency": concurrency, "body_bytes": size, "valid": True,
                "measured_request_id_prefix": prefix+"m-", "warmup_request_id_prefix": prefix+"w-",
                "requested": 4, "verified": 4, "warmup_requests": 4, "failure_count": 0, "warmup_elapsed_ns": 1000, "elapsed_ns": elapsed,
                "requests_per_second": 4e9/elapsed, "body_mib_per_second": 4e9/elapsed*size/1048576,
                "new_connections": 0, "reused_connections": 4,
                "checks": {"every_body_compared": True, "every_body_sha256_checked": True, "unique_request_ids": True, "cache_metadata_absent": True, "render_error_count_zero_checked": True, "request_id_header_present": True, "request_id_occurrences": 3, "cache_control": "no-store", "expected_status": 200, "expected_media_type": "text/html", "verified_measured_body_bytes": 4*size},
                "latency_ns": [1000, 4000, 2000, 3000], "latency": {"sample_count": 4, "average_ns": 2500, "min_ns": 1000, "max_ns": 4000, "p50_ns": 2000, "p95_ns": 4000, "p99_ns": 4000}})
    return run


def change_fingerprint(run, field):
    run["provenance"][field]["files"][0]["sha256"] = "f"*64
    raw = json.dumps(run["provenance"][field]["files"], separators=(",", ":")).encode()
    run["provenance"][field]["sha256"] = hashlib.sha256(raw).hexdigest()


class SSRComparisonTests(unittest.TestCase):
    def test_engine_identity_and_correct_statistics_with_a_fixed_runner(self):
        before, after = valid_run(), valid_run()
        after["provenance"]["server_binary"]["build_settings"].update({"vcs.revision": "3"*40, "vcs.modified": "true"})
        after["provenance"]["server_binary"]["sha256"] = "b"*64
        for trial in after["trials"]:
            trial["elapsed_ns"] //= 2
            trial["requests_per_second"] *= 2
            trial["body_mib_per_second"] *= 2
        result = COMPARE.compare(before, after)
        self.assertEqual(result["after"]["server_revision"], "3"*40)
        self.assertEqual(result["before"]["runner_checkout_revision"], result["after"]["runner_checkout_revision"])
        self.assertEqual(len(result["rows"]), 2)
        self.assertEqual(result["rows"][0]["before"]["requests_per_second"], 1000)
        self.assertEqual(result["rows"][0]["change_percent"]["requests_per_second"], 100)
        self.assertEqual(result["rows"][0]["change_percent"]["p95_ns"], 0)

    def test_missing_server_revision_keeps_binary_identity(self):
        run = valid_run()
        del run["provenance"]["server_binary"]["build_settings"]["vcs.revision"]
        result = COMPARE.compare(run, run)
        self.assertIsNone(result["before"]["server_revision"])
        self.assertIn("not embedded", COMPARE.markdown(result, "before", "after"))

    def test_rejects_changed_runner_environment_or_workload(self):
        changes = {
            "runner binary": lambda r: r["provenance"]["runner_binary"].update(sha256="c"*64),
            "server slots": lambda r: r["options"].update(server_gomaxprocs=2),
            "Go tuning": lambda r: r["environment"]["inherited_go_tuning"].update(GOGC="50"),
            "CPU": lambda r: r["environment"].update(cpu="Apple M4"),
            "OS": lambda r: r["environment"].update(os_version="27.0"),
            "server Go": lambda r: r["provenance"]["server_binary"].update(go_version="go1.27.0"),
            "compiler flags": lambda r: r["provenance"]["server_binary"]["build_settings"].update(CGO_ENABLED="0"),
            "methodology": lambda r: r["methodology"].append("Different body validation policy"),
            "runner source": lambda r: change_fingerprint(r, "runner_source"),
            "fixture": lambda r: (change_fingerprint(r, "fixture_source"), change_fingerprint(r, "isolated_fixture")),
            "warmup": lambda r: (r["options"].update(warmup_requests_per_trial=5), [t.update(warmup_requests=5) for t in r["trials"]]),
        }
        for label, mutate in changes.items():
            with self.subTest(label=label):
                before, after = valid_run(), valid_run()
                mutate(after)
                with self.assertRaisesRegex(COMPARE.InvalidRun, "not comparable"):
                    COMPARE.compare(before, after)

    def test_rejects_missing_isolation_and_response_validation(self):
        changes = {
            "wrong workload": lambda r: r.update(benchmark="response-cache"),
            "raw profile": lambda r: r["options"].update(profile="raw"),
            "debug logging": lambda r: r["options"].update(log_level="debug"),
            "preflight skipped": lambda r: r["preflight"].update(requests=0),
            "preflight failed": lambda r: r["preflight"].update(valid=False),
            "cache body": lambda r: r["server"].update(response_cache_entries_after_run=1),
            "cached response": lambda r: r["trials"][0]["checks"].update(cache_metadata_absent=False),
            "wrong status": lambda r: r["trials"][0]["checks"].update(expected_status=500),
            "wrong content type": lambda r: r["trials"][0]["checks"].update(expected_media_type="text/plain"),
            "partial body": lambda r: r["trials"][0]["checks"].update(every_body_compared=False),
            "no SHA check": lambda r: r["trials"][0]["checks"].update(every_body_sha256_checked=False),
            "missing rid": lambda r: r["trials"][0]["checks"].update(request_id_occurrences=2),
            "no unique rid": lambda r: r["trials"][0]["checks"].update(unique_request_ids=False),
            "reused prefix": lambda r: r["trials"][1].update(measured_request_id_prefix=r["trials"][0]["measured_request_id_prefix"]),
            "warmup collision": lambda r: r["trials"][0].update(warmup_request_id_prefix=r["trials"][0]["measured_request_id_prefix"]),
            "wrong canonical hash": lambda r: r["workload"].update(canonical_template_sha256="f"*64),
            "wrong body size": lambda r: r["workload"].update(body_bytes=1234),
        }
        for label, mutate in changes.items():
            with self.subTest(label=label):
                before, after = valid_run(), valid_run()
                mutate(after)
                with self.assertRaises(COMPARE.InvalidRun):
                    COMPARE.compare(before, after)

    def test_rejects_incomplete_or_inconsistent_measurements(self):
        changes = {
            "invalid": lambda r: r.update(valid=False),
            "unfinished": lambda r: r.update(completed_at_utc="0001-01-01T00:00:00Z"),
            "unclean shutdown": lambda r: r["server"].update(shutdown_clean=False),
            "missing trial": lambda r: r["trials"].pop(),
            "duplicate trial": lambda r: r["trials"][-1].update(repeat=2),
            "missing sample": lambda r: r["trials"][0]["latency_ns"].pop(),
            "missing response": lambda r: r["trials"][0].update(verified=3),
            "failure": lambda r: r["trials"][0].update(failure_count=1),
            "missing connections": lambda r: r["trials"][0].update(reused_connections=0),
            "invented throughput": lambda r: r["trials"][0].update(requests_per_second=9000),
            "invented p99": lambda r: r["trials"][0]["latency"].update(p99_ns=3000),
            "NaN": lambda r: r["trials"][0].update(requests_per_second=float("nan")),
            "missing manifest": lambda r: r["provenance"].pop("runner_source"),
            "altered manifest": lambda r: r["provenance"]["fixture_source"]["files"][0].update(bytes=20),
        }
        for label, mutate in changes.items():
            with self.subTest(label=label):
                before, after = valid_run(), valid_run()
                mutate(after)
                with self.assertRaises(COMPARE.InvalidRun):
                    COMPARE.compare(before, after)

    def test_does_not_use_saved_aggregates_or_temporary_identifiers(self):
        before, after = valid_run(), valid_run()
        after["summary"] = [{"median_requests_per_second": 9999999}]
        after["options"].update(repository="/new/checkout", output_directory="/new/results")
        after["server"].update(address="127.0.0.1:54321", startup_ns=9876)
        for trial in after["trials"]:
            for key in ("measured_request_id_prefix", "warmup_request_id_prefix"):
                trial[key] = trial[key].replace("1234567890abcdef", "aaaaaaaaaaaaaaaa")
        result = COMPARE.compare(before, after)
        self.assertTrue(all(delta == 0 for row in result["rows"] for delta in row["change_percent"].values()))

    def test_cli_refuses_malformed_json_without_comparison_output(self):
        with tempfile.TemporaryDirectory() as folder:
            source = Path(folder)/"bad.json"
            source.write_text("{broken")
            result = subprocess.run([sys.executable, "-B", str(SCRIPT), str(source), str(source)], text=True, capture_output=True)
        self.assertEqual(result.returncode, 2)
        self.assertEqual(result.stdout, "")
        self.assertIn("Comparison refused", result.stderr)


if __name__ == "__main__":
    unittest.main()
