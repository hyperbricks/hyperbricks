#!/usr/bin/env python3
"""Exercise the project lifecycle fixture through real CLI and HTTP flows.

Python 3.9+ and Go are required. --binary can select a binary built from this
checkout; without it the check builds one. No repository output is generated.
"""

import argparse
from concurrent.futures import ThreadPoolExecutor
from contextlib import contextmanager
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
MODULE = ROOT / "modules/project-lifecycle-test"
NAME = "project-lifecycle-test"
spec = importlib.util.spec_from_file_location("stage_source", MODULE / "tools/stage_source.py")
staging = importlib.util.module_from_spec(spec)
spec.loader.exec_module(staging)


def check(condition, message):
    if not condition:
        raise AssertionError(message)


def run(args, cwd, log, env=None):
    with log.open("w") as output:
        result = subprocess.run([str(arg) for arg in args], cwd=cwd, env=env,
                                stdout=output, stderr=subprocess.STDOUT, timeout=180)
    check(result.returncode == 0, f"Command failed; see {log}")


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def request(url, data=None, headers=None):
    req = urllib.request.Request(url, data=data, headers=headers or {})
    opener = urllib.request.build_opener(NoRedirect())
    try:
        response = opener.open(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, response.headers, response.read().decode("utf-8")


@contextmanager
def server(args, cwd, log, ready_url, env=None):
    with log.open("w") as output:
        process = subprocess.Popen([str(arg) for arg in args], cwd=cwd, env=env,
                                   stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
        try:
            for _ in range(150):
                check(process.poll() is None, f"Server exited; see {log}")
                try:
                    if request(ready_url)[0] == 200:
                        break
                except (OSError, urllib.error.URLError):
                    pass
                time.sleep(0.1)
            else:
                raise AssertionError(f"Server did not become ready; see {log}")
            yield
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()


def html(base, path, fragment=False):
    status, headers, body = request(base + path)
    check(status == 200, f"{path}: HTTP {status}")
    check(headers.get("X-Hyperbricks-Render-Error-Count", "0") == "0",
          f"{path}: render error: {body[:700]}")
    if fragment:
        check("<html" not in body.lower() and "<head" not in body.lower(), f"{path}: duplicated shell")
    else:
        check("<!doctype html>" in body.lower() and "<html" in body.lower(), f"{path}: missing document")
    return headers, body


def verify_core(binary, workspace):
    project = workspace / "development"
    module = staging.stage(MODULE, project)
    port = free_port()
    base = f"http://127.0.0.1:{port}"
    with server([binary, "start", "-m", NAME, "-p", port, "--non-interactive"], project,
                workspace / "development.log", base + "/"):
        pages = {"/": "Project Lifecycle Fixture", "/projects": "Atlas", "/projects/atlas": "Atlas",
                 "/projects/beacon": "Beacon", "/estimate?quantity=3": "€59.85",
                 "/static-preview": "Static export fixture"}
        assets = set()
        for path, expected in pages.items():
            _, body = html(base, path)
            check(expected.lower() in body.lower(), f"{path}: missing {expected}")
            assets.update(re.findall(r'(?:src|href)="(/static/[^" ]+)"', body))
        for path in ("/fragments/overview", "/fragments/projects", "/fragments/projects/atlas",
                     "/fragments/projects/beacon", "/fragments/status", "/fragments/estimate?quantity=3"):
            html(base, path, fragment=True)
        check(any(path.endswith(".css") for path in assets) and any(path.endswith(".js") for path in assets),
              "Native esbuild did not provide both CSS and JavaScript")
        for asset in assets:
            status, _, body = request(base + asset)
            check(status == 200 and body, f"Asset failed: {asset}")
        print("PASS pages, shared fragments, and native assets", flush=True)

        for query in ("quantity=0", "quantity=-1", "quantity=2.5", "quantity=abc", "quantity=101",
                      "quantity=", "quantity=2&quantity=3", "quantity=%3Cscript%3E"):
            headers, body = html(base, "/estimate?" + query)
            check("Choose one whole quantity" in body and 'id="estimate-total"' not in body, query)
            check(headers.get("Cache-Control") == "no-store", "Estimate must not cache a client's result")

        def estimate(quantity):
            headers, body = html(base, f"/estimate?quantity={quantity}&unit_price_cents=1&unit_price=1")
            check(f"€{quantity * 1995 / 100:.2f}" in body, f"Wrong or mixed result for {quantity}")
            check(headers.get("Cache-Control") == "no-store", "Estimate response was cacheable")
        with ThreadPoolExecutor(max_workers=4) as pool:
            list(pool.map(estimate, range(1, 13)))
        print("PASS input validation, query allowlist, and concurrent estimates", flush=True)

        # Change a real author-facing YAML value in the disposable project.
        views = module / "hyperbricks/partials/views.hyperbricks.yaml"
        before = views.read_text()
        check("unit_price_cents: 1995" in before, "Configured estimate price moved; update this exercise")
        views.write_text(before.replace("unit_price_cents: 1995", "unit_price_cents: 2095"))
        for _ in range(60):
            time.sleep(0.2)
            if "€62.85" in request(base + "/estimate?quantity=3")[2]:
                break
        else:
            raise AssertionError("Development watcher did not reload the changed YAML value")
        views.write_text(before)
        print("PASS development reload after a YAML edit", flush=True)

        # Add explicit fixture sources while the watcher is running. Documentation
        # prose is deliberately not part of this test contract.
        shutil.copy2(module / "fixtures/about.hyperbricks.yaml",
                     module / "hyperbricks/about.hyperbricks.yaml")
        navigation = module / "hyperbricks/partials/navigation.hyperbricks.yaml"
        navigation.write_text(navigation.read_text()
                              + module.joinpath("fixtures/navigation-about.hyperbricks.yaml").read_text())
        for _ in range(60):
            time.sleep(0.2)
            if request(base + "/about")[0] == 200:
                break
        else:
            raise AssertionError("The documented page did not load after adding its source")
        _, page = html(base, "/about")
        _, fragment = html(base, "/fragments/about", fragment=True)
        check("About this fixture" in page and "About this fixture" in fragment,
              "Added page and fragment do not share their view")
        check('href="/about"' in page and page.count('id="main-content"') == 1,
              "Documented navigation or shared shell is missing")
        print("PASS development reload after adding a page, fragment, and navigation entry", flush=True)

    # Production uses the compiled rendering path; test fresh client data there too.
    port = free_port()
    base = f"http://127.0.0.1:{port}"
    with server([binary, "start", "-m", NAME, "-p", port, "--production", "--non-interactive"], project,
                workspace / "live.log", base + "/"):
        for quantity in (3, 8, 3):
            headers, body = html(base, f"/estimate?quantity={quantity}")
            check(f"€{quantity * 1995 / 100:.2f}" in body and headers.get("Cache-Control") == "no-store",
                  "Production result was stale or mixed")
    print("PASS production request-specific rendering", flush=True)


def verify_packaging(binary, workspace):
    project = workspace / "release"
    module = staging.stage(MODULE, project)
    try:
        staging.stage(MODULE, project)
    except ValueError:
        pass
    else:
        raise AssertionError("Source staging must refuse to overwrite an existing project")
    run([binary, "build", "--hra", "-m", NAME, "--non-interactive"], project, workspace / "build.log")
    archives = list((project / "deploy" / NAME).glob("*.hra"))
    check(len(archives) == 1, "Expected one newly built runtime archive")
    with zipfile.ZipFile(archives[0]) as archive:
        paths = [Path(info.filename) for info in archive.infolist() if not info.is_dir()]
        check(any(path.name == "package.hyperbricks.yaml" for path in paths), "Archive missing package config")
        for path in paths:
            check(not {"logs", "node_modules", ".git"}.intersection(path.parts)
                  and ("rendered" not in path.parts or path.name == ".gitkeep")
                  and path.name not in (".env", ".DS_Store"), f"Unwanted archive content: {path}")
    port = free_port()
    base = f"http://127.0.0.1:{port}"
    with server([binary, "start", "--deploy", "-m", NAME, "-p", port, "--non-interactive"], project,
                workspace / "deploy.log", base + "/"):
        _, body = html(base, "/estimate?quantity=3")
        check("€59.85" in body, "Runtime archive did not execute the estimate")
    print("PASS clean source staging and runtime archive startup", flush=True)

    static_project = workspace / "static"
    module = staging.stage(MODULE, static_project, static_profile=True)
    run([binary, "static", "-m", NAME, "--force", "--zip", "--non-interactive"], static_project,
        workspace / "static.log")
    rendered = module / "rendered"
    check((rendered / "index.html").is_file(), "Static fixture page missing")
    document = (rendered / "index.html").read_text()
    check("static export fixture" in document.lower() and "<form" not in document and "hx-get" not in document,
          "Static fixture page depends on runtime interaction")
    links = re.findall(r'(?:href|src)="([^"]+)"', document)
    for link in links:
        if link.startswith("/") and not link.startswith("//"):
            target = rendered / link.lstrip("/")
            check(target.exists(), f"Static page has a missing local target: {link}")
    check(list((static_project / "exports" / NAME).glob("*.zip")), "Static export zip missing")
    print("PASS isolated static profile and export archive", flush=True)


def form(base, path, values):
    status, headers, body = request(base + path, urllib.parse.urlencode(values).encode(),
                                    {"Content-Type": "application/x-www-form-urlencoded"})
    check(status == 200 and headers.get("X-Hyperbricks-Render-Error-Count", "0") == "0",
          f"Form {path} failed: HTTP {status}; {body[:500]}")
    return headers, body


def verify_api(binary, workspace):
    project = workspace / "api"
    module = staging.stage(MODULE, project)
    fixture = workspace / "fixture-api"
    run(["go", "build", "-o", fixture, module / "tools/fixture-api/main.go"], project,
        workspace / "fixture-build.log")
    api_port, port = free_port(), free_port()
    api = f"http://127.0.0.1:{api_port}"
    base = f"http://127.0.0.1:{port}"
    env = {**os.environ, "HYPERBRICKS_LIFECYCLE_API_URL": api}
    with server([fixture, "-port", api_port], project, workspace / "fixture.log", api + "/health"):
        with server([binary, "start", "-m", NAME, "--config", "package.api.hyperbricks.yaml",
                     "-p", port, "--non-interactive"], project, workspace / "api.log", base + "/advanced", env):
            _, body = html(base, "/advanced")
            check("Community garden" in body, "API content was missing from initial page render")
            html(base, "/fragments/advanced-project", fragment=True)
            for name, version, expected in (("Shared garden", 1, 200), ("", 2, 422), ("Stale name", 1, 409)):
                _, body = form(base, "/actions/advanced-project-save", {"name": name, "version": version})
                check(f'data-project-save-status="{expected}"' in body, f"Missing upstream status {expected}")
                stored = json.loads(request(api + "/api/project")[2])
                check(stored == {"name": "Shared garden", "version": 2}, "Failed save changed API state")
            print("PASS API initial rendering, save, validation, and stale-version conflict", flush=True)

            for token, redirect, expected_api in ((None, "/advanced/login", 401),
                                                   ("unknown", "/advanced/login", 401),
                                                   ("demo-viewer", "/advanced/forbidden", 403),
                                                   ("demo-owner", None, 200)):
                status, headers, body = request(base + "/advanced/settings",
                                                headers={"Cookie": f"token={token}"} if token else {})
                if redirect:
                    check(status == 303 and headers.get("Location") == redirect, f"Wrong guard result: {token}")
                else:
                    check(status == 200 and "owner-only project settings" in body, "Owner could not read settings")
                actual = request(api + "/api/settings", headers={"Authorization": f"Bearer {token}"} if token else {})[0]
                check(actual == expected_api, f"Direct service access check failed: {token}")
            for role in ("viewer", "owner"):
                headers, _ = form(base, "/actions/demo-login", {"role": role})
                cookie = headers.get("Set-Cookie", "")
                check(f"token=demo-{role}" in cookie and "HttpOnly" in cookie and "SameSite=Lax" in cookie,
                      "Demo role cookie missing or incorrectly configured")
            headers, _ = form(base, "/actions/demo-logout", {})
            check("Max-Age=0" in headers.get("Set-Cookie", ""), "Logout did not expire demo cookie")
            print("PASS demo role cookies, page guards, and direct API authorization", flush=True)


def verify_plugin(binary, workspace):
    project = workspace / "plugin"
    module = staging.stage(MODULE, project)
    env = {**os.environ, "HYPERBRICKS_LOCAL_PATH": str(ROOT)}
    run([binary, "plugin", "build", "lifecycle-test@1.0.0", "--module", NAME], project,
        workspace / "plugin-build.log", env)
    run(["go", "test", "./..."], module / "plugins/lifecycle-test/1.0.0", workspace / "plugin-test.log")
    port = free_port()
    base = f"http://127.0.0.1:{port}"
    with server([binary, "start", "-m", NAME, "--config", "package.plugin.hyperbricks.yaml",
                 "-p", port, "--non-interactive"], project, workspace / "plugin.log", base + "/advanced/workflow"):
        html(base, "/advanced/workflow")
        _, preview = form(base, "/actions/advanced-name-preview", {"name": "Shared garden", "action": "confirm"})
        check('data-review-step="preview"' in preview and "2 words" in preview,
              "Plugin preview failed or request overrode configured action")
        _, confirmed = form(base, "/actions/advanced-name-confirm", {"name": "Shared garden"})
        check('data-review-step="confirmed"' in confirmed and "Shared garden" in confirmed, "Plugin confirmation failed")
        _, invalid = form(base, "/actions/advanced-name-preview", {"name": ""})
        check('data-review-step="input"' in invalid, "Plugin accepted invalid input")
        _, escaped = form(base, "/actions/advanced-name-preview", {"name": "<b>Garden</b>"})
        check("&lt;b&gt;Garden&lt;/b&gt;" in escaped and "<b>Garden</b>" not in escaped, "Plugin result was not escaped")
    print("PASS native plugin build, direct template contract, actions, and escaping", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, help="source-matched HyperBricks binary")
    parser.add_argument("--keep", action="store_true", help="keep temporary project and logs after success")
    parser.add_argument("--with-plugin", action="store_true", help="also build and check the native Go plugin profile")
    args = parser.parse_args()
    workspace = Path(tempfile.mkdtemp(prefix="hb-lifecycle-"))
    print(f"Verification workspace: {workspace}", flush=True)
    try:
        binary = args.binary.resolve() if args.binary else workspace / "hyperbricks"
        if not args.binary:
            run(["go", "build", "-o", binary, "./cmd/hyperbricks"], ROOT, workspace / "compile.log")
        verify_core(binary, workspace)
        verify_api(binary, workspace)
        if args.with_plugin:
            verify_plugin(binary, workspace)
        verify_packaging(binary, workspace)
    except Exception:
        print(f"FAILED; retained diagnostics at {workspace}", flush=True)
        raise
    if not args.keep:
        shutil.rmtree(workspace)
    print("All project lifecycle fixture checks passed.", flush=True)


if __name__ == "__main__":
    main()
