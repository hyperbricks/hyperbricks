"""Finite, repeatable before_start task. Uses only Python's standard library."""
import json
import os
from datetime import datetime, timezone
from pathlib import Path
from uuid import uuid4

if os.environ.get("HOOKS_DEMO_FAIL_PREPARE") == "1":
    raise SystemExit("Requested preparation failure (HOOKS_DEMO_FAIL_PREPARE=1)")

runtime = Path(__file__).resolve().parent / ".runtime"
runtime.mkdir(exist_ok=True)
session = {
    "session_id": str(uuid4()),
    "prepared_at": datetime.now(timezone.utc).isoformat(),
    "module_root": os.environ.get("HB_MODULE_ROOT", str(runtime.parent.parent)),
}
(runtime / "session.json").write_text(json.dumps(session), encoding="utf-8")
(runtime / "verified.json").unlink(missing_ok=True)
print(f"Prepared session {session['session_id']}", flush=True)
