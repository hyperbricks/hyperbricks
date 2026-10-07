#!/usr/bin/env python3
"""Record hook context in the disposable module; never publish anything."""
import json
import os
from pathlib import Path

root = Path(os.environ["HB_MODULE_ROOT"])
record = {key: os.environ.get("HB_" + key.upper(), "") for key in
          ("hook_phase", "operation", "outcome", "failed_phase", "exit_code", "server_port", "export_zip")}
if record["hook_phase"] == "after_static" and record["export_zip"]:
    assert Path(record["export_zip"]).is_file(), "after_static ran before ZIP completion"
with (root / "lifecycle-events.jsonl").open("a") as stream:
    stream.write(json.dumps(record) + "\n")
