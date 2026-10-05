#!/usr/bin/env python3
"""Exercise response-cache-test through the real CLI and HTTP listener.

Python 3.9+ and Go are required. Supply --binary to reuse a checkout build.
Fixtures, cache bodies, archives, and logs stay in a disposable workspace;
failed workspaces are retained for diagnosis. No external service is needed.
"""

from __future__ import annotations

import argparse
from contextlib import contextmanager
from datetime import datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile


ROOT = Path(__file__).resolve().parents[1]
MODULE = ROOT / "modules/response-cache-test"
CACHE_HEADERS = ("ETag", "X-Hyperbricks-Rendered-At", "X-Hyperbricks-Cache-Expires-At")
OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def check(condition, message):
    if not condition:
        raise AssertionError(message)


def clean_environment():
    # Preserve the real user's home/cache directory. The temporary module's
    # canonical path gives private control discovery a distinct identity.
    env = {key: value for key, value in os.environ.items()
           if not key.startswith("HB_DEPLOY_")
           and key not in {"HB_PRODUCTION", "HB_PROCESS_TEST_MODE", "HB_RUNTIME_PROCESS_GATE_ARGS"}}
    env.update(HB_NO_KEYBOARD="1", NO_COLOR="1")
    return env


def run(command, workspace, name, env, success=True, cwd=None):
    log = workspace / (name + ".log")
    with log.open("w") as output:
        result = subprocess.run([str(part) for part in command], cwd=cwd or workspace, env=env,
                                stdout=output, stderr=subprocess.STDOUT, timeout=180)
    text = log.read_text(errors="replace")
    if success:
        check(result.returncode == 0, f"Command failed ({log}):\n{text[-3000:]}")
    return result.returncode, text


def stage(workspace, name):
    module = workspace / name / MODULE.name
    shutil.copytree(MODULE, module, ignore=shutil.ignore_patterns(
        ".cache", "generated-cache", "logs", "__pycache__"))
    return module


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def request(base, path, headers=None):
    req = urllib.request.Request(base + path, headers=headers or {})
    try:
        response = OPENER.open(req, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, response.headers, response.read().decode("utf-8")


@contextmanager
def server(binary, module, workspace, label, env, profile=None):
    port = free_port()
    base = f"http://127.0.0.1:{port}"
    command = [binary, "start", "--module", module, "--port", str(port), "--non-interactive"]
    if profile:
        command.extend(["--config", f"package.{profile}.hyperbricks.yaml"])
    log = workspace / (label + ".log")
    with log.open("w") as output:
        process = subprocess.Popen([str(part) for part in command], cwd=workspace, env=env,
                                   stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
        try:
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                check(process.poll() is None,
                      f"Server exited during startup ({log}):\n{log.read_text(errors='replace')[-3000:]}")
                try:
                    if request(base, "/healthz")[0] == 200:
                        break
                except (OSError, urllib.error.URLError):
                    pass
                time.sleep(0.05)
            else:
                raise AssertionError(f"Server did not become ready; see {log}")
            yield base
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=8)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()


@contextmanager
def upstream():
    class Handler(BaseHTTPRequestHandler):
        calls = 0

        def do_GET(self):
            check(self.path.startswith("/cache-probe"), "Unexpected mock endpoint")
            Handler.calls += 1
            query = urllib.parse.parse_qs(urllib.parse.urlsplit(self.path).query)
            body = json.dumps({"call": Handler.calls, "variant": query.get("variant", ["default"])[0]}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_args):
            pass

    httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=httpd.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{httpd.server_port}", Handler
    finally:
        httpd.shutdown()
        httpd.server_close()
        thread.join(timeout=2)


def probe(base, path, cached):
    status, headers, body = request(base, path)
    check(status == 200, f"{path}: HTTP {status}: {body[:300]}")
    check(headers.get("X-Hyperbricks-Render-Error-Count", "0") == "0", f"{path}: render error")
    check(re.search(r"rendered_at=\d+ variant=", body), f"{path}: missing render marker: {body[:300]}")
    check(headers.get("X-Cache-Fixture") == "response-cache-test", f"{path}: lost configured header")
    check(headers.get("Cache-Control") == "no-store", f"{path}: changed browser-cache policy")
    check(headers.get("Content-Type", "").startswith("text/plain"), f"{path}: changed content type")
    for name in CACHE_HEADERS:
        check(bool(headers.get(name)) == cached, f"{path}: unexpected {name}: {headers.get(name)!r}")
    return headers, body


def pair(base, path, cached=True):
    first_headers, first = probe(base, path, cached)
    second_headers, second = probe(base, path, cached)
    check((first == second) == cached, f"{path}: expected {'reused' if cached else 'fresh'} output")
    if cached:
        for name in CACHE_HEADERS:
            check(first_headers[name] == second_headers[name], f"{path}: changed cached {name}")
    return first_headers, first


def ttl(headers):
    parse = lambda value: datetime.strptime(value, "%Y-%m-%d %H:%M:%S (%z)")
    return (parse(headers[CACHE_HEADERS[2]]) - parse(headers[CACHE_HEADERS[1]])).total_seconds()


def entries(module, directory=".cache"):
    return list((module / directory / "responses").glob("**/*.entry"))


def eventually(predicate, message, seconds=4):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(0.05)
    check(predicate(), message)


def purge(binary, module, workspace, env, label, route=None):
    args = [binary, "cache", "purge", "--module", module]
    args.extend(["--route", route] if route else ["--all"])
    _, output = run(args, workspace, label, env)
    match = re.search(r"Purged (\d+) memory entries and (\d+) disk entries \((\d+) bytes\)", output)
    check(match, f"Unexpected purge output: {output}")
    return tuple(map(int, match.groups()))


def verify_live(binary, workspace, env):
    module = stage(workspace, "live")
    with upstream() as (url, handler):
        live_env = dict(env, HB_CACHE_TEST_UPSTREAM=url)
        with server(binary, module, workspace, "live", live_env) as base:
            check(not (module / ".cache").exists(), "Startup eagerly created the disk cache")
            headers, _ = pair(base, "/probe/default")
            check(ttl(headers) == 600, "Omitted route cache did not inherit live.cache=10m")
            headers, _ = pair(base, "/probe/empty")
            check(ttl(headers) == 600, "Empty route cache did not inherit memory/10m defaults")
            pair(base, "/probe/page-mem")
            for path in ("zero-scalar", "zero-mem", "zero-disk", "nocache"):
                pair(base, "/probe/" + path, cached=False)
            check(not (module / ".cache").exists(), "Memory or bypass routes created the disk cache")

            # Capture all 1s routes together, then wait once for their expiry.
            expiring = {}
            for name in ("scalar", "mem", "disk", "expire-only", "page-mem", "page-disk"):
                path = "/probe/" + name
                headers, body = pair(base, path)
                check(ttl(headers) == 1, f"{path}: did not use its route TTL")
                status, same_headers, empty = request(base, path, {"If-None-Match": headers["ETag"]})
                check(status == 304 and empty == "", f"{path}: conditional request did not return 304")
                check(same_headers.get("ETag") == headers["ETag"], f"{path}: 304 lost ETag")
                expiring[path] = body
            check(entries(module), "Disk cache did not write response bodies")
            time.sleep(1.15)
            eventually(lambda: not entries(module), "Expired disk files were not cleaned without another request")
            for path, old in expiring.items():
                check(probe(base, path, True)[1] != old, f"{path}: did not re-render after expiry")
            headers, _ = pair(base, "/probe/disk-inherit")
            check(ttl(headers) == 600, "Disk route omitted expiry did not inherit live.cache")
            print("PASS default/scalar/mem/disk policies, page/fragment hits, expiry, 304, headers, lazy disk and cleanup", flush=True)

            purge(binary, module, workspace, env, "purge-reset")
            mem_path = "/probe/default?variant=keep"
            _, keep = pair(base, mem_path)
            variants = {}
            for variant in ("one", "two"):
                path = "/probe/disk-inherit?variant=" + variant
                _, variants[path] = pair(base, path)
                check("variant=" + variant in variants[path], "Query variant was not forwarded")
            check(len(set(variants.values())) == 2, "Query variants shared a response")
            removed = purge(binary, module, workspace, env, "purge-route", "/probe/disk-inherit/")
            check(removed[0] == 0 and removed[1] == 2 and removed[2] > 0, f"Wrong route purge counts: {removed}")
            check(probe(base, mem_path, True)[1] == keep, "Route purge invalidated another route")
            check(not entries(module), "Route purge left disk bodies")
            for path, old in variants.items():
                check(probe(base, path, True)[1] != old, f"Route purge missed {path}")
            removed = purge(binary, module, workspace, env, "purge-all")
            check(removed[0] == 1 and removed[1] == 2, f"Wrong full purge counts: {removed}")
            check(probe(base, mem_path, True)[1] != keep, "Full purge missed memory entry")
            check(not entries(module), "Full purge left disk bodies")
            code, output = run([binary, "cache", "purge", "--module", module, "--route", "unknown-cache-route"],
                               workspace, "purge-unknown", env, success=False)
            check(code != 0 and "unknown" in output.lower(), "Unknown route purge was accepted")
            print("PASS query separation, route/all CLI purge, preserved unrelated route, unknown-route validation", flush=True)

            first = request(base, "/probe/api?variant=api")
            second = request(base, "/probe/api?variant=api")
            check(first[0] == second[0] == 200 and handler.calls == 2, "API fragment did not call upstream each time")
            check(first[2] != second[2] and "variant=api" in second[2],
                  f"API fragment was cached or lost query: first={first[2]!r}, second={second[2]!r}")
            check(not any(second[1].get(name) for name in CACHE_HEADERS), "API fragment entered route response cache")
            _, before_restart = pair(base, "/probe/disk-inherit?variant=restart")
            print("PASS API fragment remains request driven", flush=True)
        check(not entries(module), "Graceful shutdown left its disk response bodies")
        with server(binary, module, workspace, "restart", live_env) as base:
            check(probe(base, "/probe/disk-inherit?variant=restart", True)[1] != before_restart,
                  "Restart reused the previous runtime's disk response")
    print("PASS shutdown cleanup and empty cache after restart", flush=True)


def verify_profiles(binary, workspace, env):
    for profile in ("development", "debug", "disabled"):
        module = stage(workspace, profile)
        with server(binary, module, workspace, profile, env, profile) as base:
            for name in ("default", "empty", "scalar", "mem", "disk", "expire-only", "disk-inherit", "page-mem", "page-disk"):
                pair(base, "/probe/" + name, cached=False)
            check(not (module / ".cache").exists(), f"{profile}: bypass mode created disk cache")
        print(f"PASS {profile} profile bypasses both response-cache stores", flush=True)

    module = stage(workspace, "custom")
    with server(binary, module, workspace, "custom", env, "custom") as base:
        check(not (module / "generated-cache").exists(), "Custom cache created at startup")
        pair(base, "/probe/disk-inherit")
        check(entries(module, "generated-cache"), "Custom cache directory was not used")
        check(not (module / ".cache").exists(), "Custom cache also wrote the default directory")
    print("PASS custom cache directory", flush=True)

    module = stage(workspace, "budget")
    with server(binary, module, workspace, "budget", env, "budget") as base:
        bodies = {}
        for variant in ("one", "two", "three"):
            path = "/probe/disk-inherit?variant=" + variant
            bodies[path] = probe(base, path, True)[1]
            check(len(entries(module)) <= 2, "Disk max_entries exceeded")
        check(len(entries(module)) == 2, "Expected two entries under entry pressure")
        check(probe(base, "/probe/disk-inherit?variant=one", True)[1] != bodies["/probe/disk-inherit?variant=one"],
              "Oldest response was not evicted under entry pressure")
        purge(binary, module, workspace, env, "budget-reset")
        # Each 192-block body is 3072 bytes: two fit the entry count but cannot
        # both fit the 4096-byte budget, isolating byte pressure from count.
        for blocks in (192, 193):
            status, headers, body = request(base, f"/bench/disk?blocks={blocks}")
            check(status == 200 and len(body.encode()) == blocks * 16 and headers.get("ETag"),
                  "Byte-budget probe was not cached")
            check(len(entries(module)) == 1, "Disk byte budget did not evict the older body")
        removed = purge(binary, module, workspace, env, "budget-bytes")
        check(removed[1] == 1 and removed[2] == 193 * 16, f"Unexpected byte accounting: {removed}")
        for _ in range(2):
            status, headers, body = request(base, "/bench/disk?blocks=1024")
            check(status == 200 and len(body) == 16384, "Oversized response was not served")
            check(not any(headers.get(name) for name in CACHE_HEADERS), "Oversized body was cached")
            check(not entries(module), "Oversized response left disk files")
        check(purge(binary, module, workspace, env, "budget-oversized") == (0, 0, 0),
              "Oversized disk response silently fell back to memory caching")
    print("PASS finite disk entry/byte budgets and fresh oversized responses", flush=True)


def verify_packaging(binary, workspace, env):
    for profile, directory in (("default", ".cache"), ("custom", "generated-cache")):
        module = stage(workspace, "package-" + profile)
        if profile == "custom":
            # Build intentionally reads the default package file. Select the
            # custom configuration there in this disposable copy only.
            shutil.copy2(module / "package.custom.hyperbricks.yaml", module / "package.hyperbricks.yaml")
        for name in {".cache", directory}:
            cache = module / name / "responses" / "private-cache-body.entry"
            cache.parent.mkdir(parents=True)
            cache.write_text("must never ship")
        source = module / "resources" / "cache-smoke-source.txt"
        source.parent.mkdir(exist_ok=True)
        source.write_text("source must ship")
        for extension in ("zip", "hra"):
            out = workspace / f"artifacts-{profile}-{extension}"
            run([binary, "build", "--module", module, "--out", out, "--" + extension, "--non-interactive"],
                workspace, f"build-{profile}-{extension}", env)
            archives = list(out.glob(f"**/*.{extension}"))
            check(len(archives) == 1, f"Expected one {extension} archive for {profile}")
            with zipfile.ZipFile(archives[0]) as archive:
                paths = [Path(name) for name in archive.namelist()]
                check(all(not {".cache", directory}.intersection(path.parts) for path in paths),
                      f"{profile} {extension} archive leaked cache files")
                for expected in ("package.hyperbricks.yaml", "probes.hyperbricks.yaml", "cache-smoke-source.txt"):
                    check(any(path.name == expected for path in paths), f"Archive omitted source {expected}")
    print("PASS ZIP/HRA exclude default/custom cache directories and preserve sources", flush=True)


def verify_doctor(binary, workspace, env):
    module = stage(workspace, "doctor")
    command = [binary, "doctor", "--module", module, "--json", "--strict"]
    _, output = run(command, workspace, "doctor-valid", env)
    report = json.loads(output)
    check(report["summary"]["failed"] == report["summary"]["warnings"] == 0,
          "Valid cache fixture failed strict doctor")
    source = module / "hyperbricks/probes.hyperbricks.yaml"
    original = source.read_text()
    cases = (("storage", "{storage: mem, expire: 1s}", "{storage: banana, expire: 1s}"),
             ("expiry", "{storage: mem, expire: 1s}", "{storage: mem, expire: nonsense}"),
             ("negative-expiry", "{storage: mem, expire: 1s}", "{storage: mem, expire: -1s}"),
             ("scalar", "- cache: 1s", "- cache: nonsense"))
    for name, before, after in cases:
        check(before in original, f"Doctor fixture marker moved: {before}")
        source.write_text(original.replace(before, after, 1))
        code, output = run(command, workspace, "doctor-invalid-" + name, env, success=False)
        report = json.loads(output)
        check(code != 0 and report["summary"]["failed"] > 0, f"Doctor accepted invalid {name}")
        check(any(item["status"] == "fail" and "cache" in item["message"].lower()
                  for item in report["checks"]), f"Doctor did not identify cache error for {name}")
    source.write_text(original)
    config = module / "package.hyperbricks.yaml"
    original_config = config.read_text()
    check("max_entries: 10000" in original_config, "Doctor disk bound fixture moved")
    config.write_text(original_config.replace("max_entries: 10000", "max_entries: 0", 1))
    code, output = run(command, workspace, "doctor-invalid-budget", env, success=False)
    report = json.loads(output)
    check(code != 0 and report["summary"]["failed"] > 0, "Doctor accepted zero disk max_entries")
    print("PASS doctor rejects invalid storage, expiry, scalar duration, and disk bounds", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, help="use a HyperBricks binary built from this checkout")
    parser.add_argument("--keep-workdir", action="store_true", help="retain temporary fixtures and logs after success")
    args = parser.parse_args()
    if sys.platform not in ("darwin", "linux"):
        parser.error("cache purge integration requires macOS or Linux private Unix sockets")
    workspace = Path(tempfile.mkdtemp(prefix="hb-response-cache-"))
    env = clean_environment()
    started = time.monotonic()
    success = False
    try:
        binary = args.binary.expanduser().resolve() if args.binary else workspace / "hyperbricks"
        if not args.binary:
            run(["go", "build", "-o", binary, "./cmd/hyperbricks"], workspace, "compile", env, cwd=ROOT)
        check(binary.is_file(), f"Binary does not exist: {binary}")
        verify_live(binary, workspace, env)
        verify_profiles(binary, workspace, env)
        verify_packaging(binary, workspace, env)
        verify_doctor(binary, workspace, env)
        success = True
        print(f"PASS response-cache-test integration ({time.monotonic() - started:.1f}s)", flush=True)
        return 0
    except (AssertionError, OSError, ValueError, subprocess.SubprocessError) as error:
        print(f"FAIL response-cache-test: {error}", file=sys.stderr)
        return 1
    finally:
        if success and not args.keep_workdir:
            shutil.rmtree(workspace)
        else:
            print(f"Response-cache workspace and logs: {workspace}", flush=True)


if __name__ == "__main__":
    sys.exit(main())
