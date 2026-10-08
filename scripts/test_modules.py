#!/usr/bin/env python3
"""Start documented modules and verify their public HTTP smoke contract.

This deliberately checks only deterministic server behaviour. Browser-only
interactions remain covered by the module README/manual walkthroughs.
"""

from __future__ import annotations

import argparse
import os
import signal
import subprocess
import sys
import socket
import time
import urllib.error
import urllib.request
from dataclasses import dataclass


ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))


@dataclass(frozen=True)
class ModuleCheck:
    name: str
    port: int
    paths: tuple[tuple[str, tuple[str, ...]], ...]


CHECKS = (
    ModuleCheck("api-security-test", 8105, (("/dashboard", ("user", "admin", "public")),)),
    ModuleCheck("esbuild-demo", 8097, (("/", ("VAT",)),)),
    ModuleCheck("spaces-image-demo", 8134, (
        ("/", ("First study", "Objects in good light", "/static/css/site.css", "_w720_h720.jpg")),
        ("/second", ("Second study", "Cream &amp; brass", "/static/css/site.css", "_w720_h720.jpg")),
    )),
    ModuleCheck("navigation-demo-swup", 8125, (("/", ("After Hours",)), ("/last-bite", ("Last Bite",)))),
    ModuleCheck("sampleapis-coffee-static", 8080, (("/", ("Coffee",)),)),
    ModuleCheck("todo-demo-htmx", 8121, (("/", ("Tasks",)), ("/fragments/tasks", ("task",)))),
    ModuleCheck("todo-demo-swup", 8124, (("/", ("Tasks",)),)),
    ModuleCheck("todo-demo-turbo", 8122, (("/", ("Tasks",)),)),
    ModuleCheck("todo-demo-unpoly", 8123, (("/", ("Tasks",)),)),
    ModuleCheck("unpoly-guard-demo", 8132, (("/", ("Sign in",)), ("/private", ())),),
    ModuleCheck("streaming-demo", 18110, (("/", ("Streaming",)),)),
    ModuleCheck(
        "yaml-nested-inheritance-test",
        8133,
        (
            ("/", ("Nested YAML inheritance regression fixture",)),
            (
                "/reference",
                ("Reference content", "Version: 1.0.0", "Channel: stable", "Override: inherited default", "Shell sibling preserved"),
            ),
            (
                "/copied",
                ("Reference content", "Version: 1.0.0", "Channel: stable", "Override: inherited default", "Shell sibling preserved"),
            ),
            (
                "/overridden",
                ("Nested inheritance override", "Version: 1.0.0", "Channel: preview", "Override: works", "Shell sibling preserved"),
            ),
            (
                "/direct-merge",
                ("Direct same-name merge", "Version: 1.0.0", "Channel: candidate", "Override: normal merge works", "Shell sibling preserved"),
            ),
            (
                "/chained",
                ("Chained dotted inheritance", "Version: 2.0.0", "Channel: preview", "Override: works", "Shell sibling preserved"),
            ),
            (
                "/deep-dotted",
                ("Deep dotted lookup", "Owner: core", "Stage: customized", "Shell sibling preserved"),
            ),
        ),
    ),
)


def fetch(url: str) -> tuple[int, str]:
    with urllib.request.urlopen(url, timeout=8) as response:
        return response.status, response.read().decode("utf-8", "replace")


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def stop_process_group(process):
    # go run may exit before its compiled child; signal the entire session.
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait()


def check_module(check: ModuleCheck) -> list[str]:
    runtime_port = check.port if check.name == "api-security-test" else free_port()
    env = os.environ.copy()
    env["HYPERBRICKS_LOCAL_PATH"] = ROOT
    env["GOWORK"] = "off"
    command = ["go", "run", "./cmd/hyperbricks", "start", "-m", check.name,
               "--port", str(runtime_port), "--non-interactive"]
    fixture = None
    if check.name == "api-security-test":
        fixture = subprocess.Popen(
            ["go", "run", "./modules/api-security-test/tools/mock-api", "-port", "8098", "-redirect-port", "8099"],
            cwd=ROOT, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            start_new_session=True,
        )
        time.sleep(1)
    process = subprocess.Popen(command, cwd=ROOT, env=env, stdout=subprocess.PIPE,
                               stderr=subprocess.STDOUT, text=True, start_new_session=True)
    base = f"http://127.0.0.1:{runtime_port}"
    errors: list[str] = []
    try:
        deadline = time.time() + 30
        while time.time() < deadline:
            try:
                fetch(base + "/")
                break
            except urllib.error.HTTPError:
                # A documented module may intentionally not expose `/`; an
                # HTTP response still proves that the listener is ready.
                break
            except (urllib.error.URLError, ConnectionError):
                if process.poll() is not None:
                    break
                time.sleep(0.25)
        else:
            errors.append("server did not become ready")
        if process.poll() is not None and not errors:
            output = (process.stdout.read() if process.stdout else "").strip()
            errors.append(f"server exited during startup: {output[-500:]}")
        for path, markers in check.paths:
            try:
                status, body = fetch(base + path)
            except Exception as exc:  # noqa: BLE001 - report the module failure
                errors.append(f"GET {path}: {exc}")
                continue
            if status != 200:
                errors.append(f"GET {path}: expected 200, got {status}")
            for marker in markers:
                if marker.lower() not in body.lower():
                    errors.append(f"GET {path}: missing marker {marker!r}")
    finally:
        stop_process_group(process)
        if fixture is not None:
            stop_process_group(fixture)
    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--module", action="append", help="run only this module (repeatable)")
    args = parser.parse_args()
    selected = {item for item in args.module} if args.module else None
    failures = 0
    for check in CHECKS:
        if selected is not None and check.name not in selected:
            continue
        errors = check_module(check)
        if errors:
            failures += 1
            print(f"FAIL {check.name}")
            for error in errors:
                print(f"  - {error}")
        else:
            print(f"PASS {check.name}")
    if selected is None or "response-cache-test" in selected:
        result = subprocess.run(
            [sys.executable, os.path.join(ROOT, "scripts", "test_response_cache_module.py")],
            cwd=ROOT,
        )
        if result.returncode:
            failures += 1
    print()
    print("-" * 72)
    if failures:
        print(f"{failures} module(s) failed")
        return 1
    print("All documented module smoke checks passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
