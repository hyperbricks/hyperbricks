#!/usr/bin/env python3
"""Exercise the real CLI lifecycle with temporary module copies and private ports.

Only Python's standard library is required. Every started CLI/child group is
recorded and cleanup is bounded, including when an assertion fails.
"""
import argparse
import contextlib
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
from urllib.request import ProxyHandler, build_opener


OPENER = build_opener(ProxyHandler({}))
MODULE = Path(__file__).resolve().parents[1]


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def eventually(check, description, timeout=15):
    deadline = time.monotonic() + timeout
    last_error = None
    while time.monotonic() < deadline:
        try:
            result = check()
            if result:
                return result
        except (OSError, ValueError, AssertionError) as error:
            last_error = error
        time.sleep(0.05)
    raise AssertionError(f"Timed out waiting for {description}; last result: {last_error}")


def free_ports():
    # Keep both reservations open until two distinct addresses have been chosen.
    with contextlib.ExitStack() as stack:
        sockets = [stack.enter_context(socket.socket()) for _ in range(2)]
        for connection in sockets:
            connection.bind(("127.0.0.1", 0))
        return [connection.getsockname()[1] for connection in sockets]


def listener_closed(port):
    with socket.socket() as connection:
        connection.settimeout(0.2)
        return connection.connect_ex(("127.0.0.1", port)) != 0


def group_exists(group):
    try:
        os.killpg(group, 0)
        return True
    except ProcessLookupError:
        return False


class Session:
    def __init__(self, binary, work, extra_env=None):
        self.module = work / MODULE.name
        shutil.copytree(MODULE, self.module, ignore=shutil.ignore_patterns(".runtime", "__pycache__"))
        self.site_port, self.api_port = free_ports()
        self.log_path = work / "session.log"
        self.log = self.log_path.open("wb")
        self.groups = set()
        self.process = None
        environment = {
            key: value for key, value in os.environ.items()
            if not key.startswith("HOOKS_DEMO_FAIL_")
        }
        environment["HOOKS_DEMO_API_PORT"] = str(self.api_port)
        environment.update(extra_env or {})
        try:
            self.process = subprocess.Popen(
                [str(binary), "start", "-m", str(self.module), "--port", str(self.site_port),
                 "--with-processes", "--non-interactive", "--log-format", "json"],
                cwd=work, env=environment, stdin=subprocess.DEVNULL,
                stdout=self.log, stderr=subprocess.STDOUT, start_new_session=True,
            )
        except BaseException:
            self.log.close()
            raise

    def records(self):
        records = []
        for line in self.log_path.read_text(encoding="utf-8", errors="replace").splitlines():
            try:
                value = json.loads(line)
            except ValueError:
                continue
            if not isinstance(value, dict):
                continue
            # HyperBricks uses "message"; accept Zap's default key as well.
            value["msg"] = value.get("message", value.get("msg"))
            records.append(value)
            if (value.get("msg") == "started" and value.get("phase") in
                    ("before_start", "after_start", "service") and
                    value.get("name") in ("prepare-demo", "verify-demo", "demo-api") and
                    isinstance(value.get("pid"), int)):
                self.groups.add(value["pid"])
        return records

    def output(self):
        return self.log_path.read_text(encoding="utf-8", errors="replace")[-16384:]

    def request(self, path="/", api=False):
        port = self.api_port if api else self.site_port
        with OPENER.open(f"http://127.0.0.1:{port}{path}", timeout=0.5) as response:
            require(response.status == 200, f"Unexpected HTTP {response.status}")
            if not api:
                require(response.headers.get("X-Hyperbricks-Render-Error-Count", "0") == "0",
                        "The page has render diagnostics")
            return response.read().decode("utf-8")

    def ready(self):
        def check():
            require(self.process.poll() is None, f"CLI exited during startup: {self.process.returncode}")
            verified_path = self.module / "demo-api/.runtime/verified.json"
            if not verified_path.exists():
                return None
            verified = json.loads(verified_path.read_text())
            if not any(row.get("name") == "verify-demo" and row.get("msg") == "completed"
                       for row in self.records()):
                return None
            data = json.loads(self.request("/message", api=True))
            require(data["session_id"] == verified["session_id"], "Verification used another session")
            require(data["site_url"] == f"http://127.0.0.1:{self.site_port}", "CLI port override was lost")
            require(data["session_id"] in self.request(), "Rendered page lacks API session data")
            self.groups.add(data["pid"])
            return data
        return eventually(check, "successful after_start verification")

    def wait(self, expected):
        code = self.process.wait(timeout=20)
        self.records()
        require(code == expected, f"Expected CLI exit {expected}, got {code}")
        eventually(lambda: listener_closed(self.site_port) and listener_closed(self.api_port),
                   "both owned listeners to close", timeout=5)
        eventually(lambda: not any(group_exists(group) for group in self.groups),
                   "owned child process groups to disappear", timeout=5)

    def cleanup(self):
        """Always reap our parent and terminate only recorded, owned groups."""
        try:
            self.records()
            if self.process is not None and self.process.poll() is None:
                self.process.send_signal(signal.SIGTERM)
                try:
                    self.process.wait(timeout=18)
                except subprocess.TimeoutExpired:
                    os.killpg(self.process.pid, signal.SIGKILL)
                    self.process.wait(timeout=3)
            # Read again: a child may have been spawned while termination began.
            self.records()
            remaining = [group for group in self.groups if group_exists(group)]
            for group in remaining:
                with contextlib.suppress(ProcessLookupError):
                    os.killpg(group, signal.SIGTERM)
            deadline = time.monotonic() + 2
            while remaining and time.monotonic() < deadline:
                remaining = [group for group in remaining if group_exists(group)]
                time.sleep(0.05)
            for group in remaining:
                with contextlib.suppress(ProcessLookupError):
                    os.killpg(group, signal.SIGKILL)
            if remaining:
                eventually(lambda: not any(group_exists(group) for group in remaining),
                           "forced child cleanup", timeout=3)
        finally:
            self.log.close()


@contextlib.contextmanager
def session(binary, extra_env=None):
    with tempfile.TemporaryDirectory(prefix="hyperbricks-hooks-smoke-") as temporary:
        running = Session(binary, Path(temporary), extra_env)
        try:
            yield running
        except BaseException:
            print("\n--- Lifecycle log (bounded tail) ---\n" + running.output(), file=sys.stderr)
            raise
        finally:
            running.cleanup()


def successful_session(binary):
    with session(binary) as running:
        initial = running.ready()
        template = running.module / "templates/page.html"
        marker = "smoke-reload-proved"
        template.write_text(template.read_text() + f"\n<p>{marker}</p>\n", encoding="utf-8")
        eventually(lambda: marker in running.request(), "watched template update")
        current = json.loads(running.request("/message", api=True))
        require(current["pid"] == initial["pid"], "Reload started a different API process")
        require(current["session_id"] == initial["session_id"], "Reload reran preparation")
        preparation = json.loads((running.module / "demo-api/.runtime/session.json").read_text())
        require(preparation["session_id"] == initial["session_id"], "Preparation output changed on reload")
        running.process.send_signal(signal.SIGINT)
        running.wait(130)
    print("PASS: page, CLI port override, source reload, stable API/session, SIGINT and cleanup")


def preparation_failure(binary):
    with session(binary, {"HOOKS_DEMO_FAIL_PREPARE": "1"}) as running:
        running.wait(1)
        require(not (running.module / "demo-api/.runtime/session.json").exists(),
                "A failing preparation unexpectedly wrote session data")
        require(not any(row.get("phase") == "service" and row.get("msg") == "started"
                        for row in running.records()), "A service started after failed preparation")
    print("PASS: preparation failure prevents service/server startup")


def verification_failure(binary):
    with session(binary, {"HOOKS_DEMO_FAIL_VERIFY": "1"}) as running:
        running.wait(1)
        require(any(row.get("name") == "demo-api" and row.get("msg") == "started"
                    for row in running.records()), "Verification scenario never started its API")
        require(not (running.module / "demo-api/.runtime/verified.json").exists(),
                "A failing verification unexpectedly wrote a success marker")
    print("PASS: after-start failure closes HTTP and cleans its API")


def service_exit(binary):
    with session(binary) as running:
        initial = running.ready()
        # The response's session ID was verified before recording this owned PID.
        os.kill(initial["pid"], signal.SIGTERM)
        running.wait(1)
        require("exited unexpectedly" in running.output(), "Missing required-service exit diagnostic")
    print("PASS: required service exit fails the session and cleans all listeners")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path, help="Binary built from this HyperBricks checkout")
    args = parser.parse_args()
    if sys.platform not in ("darwin", "linux"):
        parser.error("This lifecycle suite requires macOS or Linux")
    binary = args.binary.expanduser().resolve()
    if not binary.is_file() or not os.access(binary, os.X_OK):
        parser.error("--binary must be an executable file")
    for scenario in (successful_session, preparation_failure, verification_failure, service_exit):
        scenario(binary)
    print("All development-hooks smoke scenarios passed; no owned servers remain.")


if __name__ == "__main__":
    main()
