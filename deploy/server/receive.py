#!/usr/bin/env python3
"""Forced SSH command for GitHub Actions: accepts one bounded source archive."""
import os
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile

ROOT = Path("/opt/smart-credit")
os.umask(0o077)
match = re.fullmatch(r"deploy ([0-9a-f]{40})", os.environ.get("SSH_ORIGINAL_COMMAND", ""))
if not match:
    print("Only 'deploy <commit SHA>' is permitted.")
    raise SystemExit(1)
sha = match.group(1)
incoming = ROOT / "incoming"
incoming.mkdir(mode=0o700, exist_ok=True)
with tempfile.NamedTemporaryFile(prefix=sha + "-", suffix=".tar", dir=incoming, delete=False) as archive:
    size = 0
    while chunk := sys.stdin.buffer.read(1024 * 1024):
        size += len(chunk)
        if size > 100 * 1024 * 1024:
            print("Source archive exceeds 100 MiB.")
            raise SystemExit(1)
        archive.write(chunk)
archive_path = Path(archive.name)
try:
    # Keep the source on disk and log to the journal, not the SSH pipe. This lets
    # the service finish even if the runner disconnects while deployment runs.
    result = subprocess.run([
        "systemd-run", "--quiet", "--wait", "--collect",
        "--unit=smart-credit-deploy-" + sha[:12] + "-" + str(os.getpid()),
        "--property=UMask=0077", "--property=RuntimeMaxSec=30min",
        "/usr/bin/python3", str(ROOT / "deployer/deploy.py"),
        "--archive", str(archive_path), "--sha", sha, "--retry",
    ])
    state_file = ROOT / "deploy-state.json"
    if state_file.exists():
        state = json.loads(state_file.read_text())
        print("Deployment status: " + state.get("status", "unknown"), flush=True)
        print("Active revision: " + state.get("active_sha", "unknown"), flush=True)
        print("Server log: " + state.get("log", "unknown"), flush=True)
    raise SystemExit(result.returncode)
finally:
    archive_path.unlink(missing_ok=True)
