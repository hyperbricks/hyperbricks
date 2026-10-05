"""Comparison safety tests; no server or timed benchmark is started."""

from copy import deepcopy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("compare-response-cache.py")
SPEC = importlib.util.spec_from_file_location("compare_response_cache", SCRIPT)
COMPARE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(COMPARE)


def valid_run():
    files = [{"path": "fixture.txt", "bytes": 16, "sha256": hashlib.sha256(b"0123456789abcdef").hexdigest(), "mode": 420}]
    manifest = {"files": files, "sha256": hashlib.sha256(json.dumps(files, separators=(",", ":")).encode()).hexdigest()}
    binary = {"sha256": "a"*64, "bytes": 100, "go_version": "go1.26.1", "build_settings": {"GOOS": "darwin", "GOARCH": "arm64", "-compiler": "gc", "CGO_ENABLED": "1", "vcs.revision": "1"*40, "vcs.modified": "false"}}
    run = {
        "format_version": 1, "valid": True,
        "started_at_utc": "2026-10-05T12:00:00Z", "completed_at_utc": "2026-10-05T12:01:00Z",
        "options": {"requests_per_trial": 4, "warmup_requests_per_trial": 4, "repeats": 3, "body_sizes_bytes": [16], "concurrency": [1], "client_gomaxprocs": 4, "server_gomaxprocs": 4, "request_timeout_ns": 10000000000, "startup_timeout_ns": 30000000000},
        "environment": {"os": "darwin", "architecture": "arm64", "os_version": "26.6.2", "kernel": "Darwin 25.6.0", "cpu": "Apple M3", "logical_cpus": 8, "client_go_version": "go1.26.1", "client_gomaxprocs": 4, "inherited_go_tuning": {}},
        "provenance": {"git_revision": "2"*40, "git_dirty": False, "server_binary": deepcopy(binary), "runner_binary": deepcopy(binary), "fixture_source": deepcopy(manifest), "isolated_fixture": deepcopy(manifest), "runner_source": deepcopy(manifest)},
        "methodology": ["Test fixture: four validated samples per trial."],
        "server": {"shutdown_clean": True, "namespaces_after_shutdown": 0, "disk_entries_before_shutdown": 1, "disk_body_bytes_before_shutdown": 16},
        "trials": [],
    }
    for repeat, elapsed in enumerate((4000000, 2000000, 8000000), 1):
        for backend in ("fresh", "mem", "disk"):
            run["trials"].append({"sequence": len(run["trials"])+1, "repeat": repeat, "body_bytes": 16, "concurrency": 1, "backend": backend, "valid": True,
                "requested": 4, "verified": 4, "warmup_requests": 4, "failure_count": 0, "warmup_elapsed_ns": 1000, "elapsed_ns": elapsed,
                "requests_per_second": 4e9/elapsed, "body_mib_per_second": 4e9/elapsed*16/1048576,
                "new_connections": 0, "reused_connections": 4,
                "checks": {"every_body_compared": True, "every_body_sha256_checked": True, "render_error_count_zero_checked": True, "expected_status": 200, "expected_media_type": "text/plain", "verified_measured_body_bytes": 64, "expected_body_sha256": files[0]["sha256"], "cached_metadata_stable": backend != "fresh"},
                "latency_ns": [1000, 4000, 2000, 3000], "latency": {"sample_count": 4, "average_ns": 2500, "min_ns": 1000, "max_ns": 4000, "p50_ns": 2000, "p95_ns": 4000, "p99_ns": 4000}})
    return run


def change_fingerprint(run, field):
    run["provenance"][field]["files"][0]["sha256"] = "f"*64
    raw = json.dumps(run["provenance"][field]["files"], separators=(",", ":")).encode()
    run["provenance"][field]["sha256"] = hashlib.sha256(raw).hexdigest()


class ComparisonTests(unittest.TestCase):
    def test_go_rfc3339_nano_fractional_widths_and_nanosecond_order(self):
        for width in range(1, 10):
            fraction = "123456789"[:width]
            with self.subTest(width=width):
                value = COMPARE.parse_timestamp("2026-10-05T12:00:00."+fraction+"Z")
                self.assertEqual(value[1], int(fraction.ljust(9, "0")))
                run = valid_run()
                run["started_at_utc"] = "2026-10-05T12:00:00."+fraction+"Z"
                self.assertTrue(COMPARE.compare(run, run)["comparable"])
        self.assertEqual(COMPARE.parse_timestamp("2026-10-05T12:00:00Z")[1], 0)
        self.assertEqual(COMPARE.parse_timestamp("2026-10-05T14:00:00.1+02:00"),
                         COMPARE.parse_timestamp("2026-10-05T12:00:00.100000000Z"))
        self.assertLess(COMPARE.parse_timestamp("2026-10-05T12:00:00.000000001Z"),
                        COMPARE.parse_timestamp("2026-10-05T12:00:00.000000002Z"))
        for invalid in ("2026-10-05T12:00:00", "2026-10-05T12:00:00.1234567890Z"):
            with self.assertRaises(COMPARE.InvalidRun):
                COMPARE.parse_timestamp(invalid)

    def test_server_identity_differs_from_runner_checkout(self):
        before, after = valid_run(), valid_run()
        after["provenance"]["server_binary"]["build_settings"].update({"vcs.revision": "3"*40, "vcs.modified": "true", "vcs.time": "later"})
        after["provenance"]["server_binary"]["sha256"] = "b"*64
        after["provenance"]["git_dirty"] = True
        for trial in after["trials"]:
            trial["elapsed_ns"] //= 2
            trial["requests_per_second"] *= 2
            trial["body_mib_per_second"] *= 2
        result = COMPARE.compare(before, after)
        self.assertEqual(result["after"]["server_revision"], "3"*40)
        self.assertEqual(result["before"]["runner_checkout_revision"], result["after"]["runner_checkout_revision"])
        self.assertEqual(result["rows"][0]["before"]["requests_per_second"], 1000)
        self.assertEqual(result["rows"][0]["change_percent"]["requests_per_second"], 100)
        self.assertEqual(result["rows"][0]["change_percent"]["p99_ns"], 0)

    def test_missing_server_revision_uses_hash_without_inventing_revision(self):
        run = valid_run()
        del run["provenance"]["server_binary"]["build_settings"]["vcs.revision"]
        result = COMPARE.compare(run, run)
        self.assertIsNone(result["before"]["server_revision"])
        self.assertIn("not embedded", COMPARE.markdown(result, "before", "after"))

    def test_different_runner_binary_is_rejected_even_when_source_manifest_matches(self):
        before, after = valid_run(), valid_run()
        after["provenance"]["runner_binary"]["sha256"] = "c"*64
        with self.assertRaisesRegex(COMPARE.InvalidRun, "runner_binary.sha256"):
            COMPARE.compare(before, after)

    def test_rejects_incompatible_workload_environment_or_method(self):
        changes = {
            "server slots": lambda r: r["options"].update(server_gomaxprocs=2),
            "Go tuning": lambda r: r["environment"]["inherited_go_tuning"].update(GOGC="50"),
            "CPU": lambda r: r["environment"].update(cpu="Apple M4"),
            "OS": lambda r: r["environment"].update(os_version="27.0"),
            "server Go": lambda r: r["provenance"]["server_binary"].update(go_version="go1.27.0"),
            "compiler flags": lambda r: r["provenance"]["server_binary"]["build_settings"].update(CGO_ENABLED="0"),
            "methodology": lambda r: r["methodology"].append("Different response verification policy"),
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

    def test_rejects_incomplete_failed_or_inconsistent_evidence(self):
        changes = {
            "invalid": lambda r: r.update(valid=False),
            "error": lambda r: r.update(error="interrupted"),
            "unfinished": lambda r: r.update(completed_at_utc="0001-01-01T00:00:00Z"),
            "unclean shutdown": lambda r: r["server"].update(shutdown_clean=False),
            "missing disk body": lambda r: r["server"].update(disk_entries_before_shutdown=0),
            "missing trial": lambda r: r["trials"].pop(),
            "duplicate trial": lambda r: r["trials"][-1].update(repeat=2),
            "missing sample": lambda r: r["trials"][0]["latency_ns"].pop(),
            "zero sample": lambda r: r["trials"][0]["latency_ns"].__setitem__(0, 0),
            "missing response": lambda r: r["trials"][0].update(verified=3),
            "request failure": lambda r: r["trials"][0].update(failure_count=1),
            "hash unchecked": lambda r: r["trials"][0]["checks"].update(every_body_sha256_checked=False),
            "wrong body hash": lambda r: r["trials"][0]["checks"].update(expected_body_sha256="f"*64),
            "invented throughput": lambda r: r["trials"][0].update(requests_per_second=9000),
            "invented p99": lambda r: r["trials"][0]["latency"].update(p99_ns=3000),
            "NaN": lambda r: r["trials"][0].update(requests_per_second=float("nan")),
            "missing metadata": lambda r: r["provenance"].pop("runner_source"),
            "altered manifest": lambda r: r["provenance"]["fixture_source"]["files"][0].update(bytes=20),
        }
        for label, mutate in changes.items():
            with self.subTest(label=label):
                before, after = valid_run(), valid_run()
                mutate(after)
                with self.assertRaises(COMPARE.InvalidRun):
                    COMPARE.compare(before, after)

    def test_ignores_aggregate_summary_and_temporary_paths(self):
        before, after = valid_run(), valid_run()
        after["summary"] = [{"median_requests_per_second": 9999999}]
        after["options"].update(repository="/new/checkout", output_directory="/new/results")
        after["server"].update(address="127.0.0.1:54321", startup_ns=9876)
        result = COMPARE.compare(before, after)
        self.assertEqual(result["rows"][0]["change_percent"]["requests_per_second"], 0)

    def test_cli_refuses_malformed_json_and_emits_no_comparison(self):
        with tempfile.TemporaryDirectory() as folder:
            source = Path(folder)/"bad.json"
            source.write_text("{broken")
            result = subprocess.run([sys.executable, str(SCRIPT), str(source), str(source)], text=True, capture_output=True)
        self.assertEqual(result.returncode, 2)
        self.assertEqual(result.stdout, "")
        self.assertIn("Comparison refused", result.stderr)


if __name__ == "__main__":
    unittest.main()
