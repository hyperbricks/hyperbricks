"""Finite after_start task: verify the full page includes this session's API data."""
import json
import os
from pathlib import Path
from urllib.request import ProxyHandler, build_opener

if os.environ.get("HOOKS_DEMO_FAIL_VERIFY") == "1":
    raise SystemExit("Requested verification failure (HOOKS_DEMO_FAIL_VERIFY=1)")

runtime = Path(__file__).resolve().parent / ".runtime"
session = json.loads((runtime / "session.json").read_text(encoding="utf-8"))
site_url = f"http://127.0.0.1:{os.environ['HB_SERVER_PORT']}"
with build_opener(ProxyHandler({})).open(site_url + "/", timeout=5) as response:
    body = response.read().decode("utf-8")
    if response.status != 200:
        raise SystemExit(f"Page verification failed: HTTP {response.status}")
    if response.headers.get("X-Hyperbricks-Render-Error-Count", "0") != "0":
        raise SystemExit("Page verification failed: HyperBricks reported render diagnostics")
if session["session_id"] not in body or "Your managed API is ready." not in body:
    raise SystemExit("Page verification failed: expected session API data is missing")
if site_url not in body:
    raise SystemExit("Page verification failed: API did not receive the effective site port")
(runtime / "verified.json").write_text(
    json.dumps({"session_id": session["session_id"], "site_url": site_url}), encoding="utf-8"
)
print(f"Verified rendered session {session['session_id']} at {site_url}", flush=True)
