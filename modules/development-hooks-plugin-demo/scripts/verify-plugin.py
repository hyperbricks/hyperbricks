"""Prove the artifact just built by before_start is the one the runtime loaded."""
import os
from pathlib import Path
from urllib.request import ProxyHandler, build_opener

build_id = (Path(os.environ["HB_MODULE_ROOT"]) / "plugins/hook-greeting/1.0.0/build-id.txt").read_text(encoding="utf-8").strip()
url = f"http://127.0.0.1:{os.environ['HB_SERVER_PORT']}/"
with build_opener(ProxyHandler({})).open(url, timeout=5) as response:
    body = response.read().decode("utf-8")
    if response.status != 200 or response.headers.get("X-Hyperbricks-Render-Error-Count", "0") != "0":
        raise SystemExit("Plugin verification failed: HTTP or render diagnostic")
if "Hooks plugin built before load: " + build_id not in body:
    raise SystemExit("Plugin verification failed: the loaded artifact has the wrong build ID")
print(f"Verified freshly compiled plugin {build_id}", flush=True)
