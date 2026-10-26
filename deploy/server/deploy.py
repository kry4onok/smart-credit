#!/usr/bin/env python3
"""Single-host main deployment. Infrastructure and secrets live outside Git."""
import fcntl
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import time

ROOT = Path(os.environ.get("DEPLOY_ROOT", "/opt/smart-credit"))
SHARED = ROOT / "shared"
INSTALL = ROOT / "deployer"
STATE = ROOT / "deploy-state.json"
SERVICES = ["auth", "bank-service", "client-service", "bureau-mocks", "scoring", "assistant", "web", "ops"]
settings = {}
logfile = None


def atomic(path, data, mode=0o600):
    temp = path.with_name(path.name + ".tmp")
    temp.write_text(data)
    temp.chmod(mode)
    temp.replace(path)


def read_env(path):
    result = {}
    for line in path.read_text().splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        key, value = line.split("=", 1)
        if not re.fullmatch(r"[A-Z][A-Z0-9_]*", key):
            raise RuntimeError("Unsupported environment syntax")
        result[key] = value
    return result


def redact(value):
    for key, secret in settings.items():
        if secret and any(word in key for word in ("PASSWORD", "TOKEN", "SECRET", "API_KEY", "DATABASE_URL")):
            value = value.replace(secret, "[REDACTED]")
    return value


def run(args, cwd=None, env=None, timeout=1200, quiet=False):
    result = subprocess.run([str(a) for a in args], cwd=cwd, env=env, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
    output = redact(result.stdout)
    if logfile and not quiet:
        with logfile.open("a") as stream:
            stream.write(output)
    if result.returncode:
        raise RuntimeError(f"{Path(str(args[0])).name} failed; see protected deployment log")
    return result.stdout.strip()


def phase(message):
    print(message, flush=True)
    if logfile:
        with logfile.open("a") as stream:
            stream.write(message + "\n")


def extract_source(archive, release, sha):
    with tarfile.open(archive) as source:
        if source.pax_headers.get("comment") != sha:
            raise RuntimeError("Archive revision does not match deployment revision")
        # The data filter intentionally drops directory modes. A secret-safe 077
        # process umask would make COPY'ed source unreadable to non-root images.
        previous_mask = os.umask(0o022)
        try:
            source.extractall(release, filter="data")
        finally:
            os.umask(previous_mask)


def apply_patches(release, patches):
    previous_mask = os.umask(0o022)
    try:
        for patch in sorted(patches):
            reverse = subprocess.run(["git", "apply", "--reverse", "--check", str(patch)], cwd=release,
                                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if reverse.returncode == 0:
                continue
            run(["git", "apply", "--check", patch], cwd=release)
            run(["git", "apply", patch], cwd=release)
    finally:
        os.umask(previous_mask)


def merge_env(release):
    global settings
    existing = read_env(SHARED / "runtime.env")
    defaults = read_env(release / ".env.example")
    # Existing values, including intentionally empty optional values, always win.
    settings = defaults | existing
    required = ["POSTGRES_SUPERUSER_PASSWORD", "AUTH_DB_PASSWORD", "CLIENT_DB_PASSWORD", "BANK_DB_PASSWORD",
                "AUTH_DEV_CLIENT_PASSWORD", "AUTH_DEV_ANALYST_PASSWORD", "AUTH_DEV_ADMIN_PASSWORD"]
    for key in required:
        if not existing.get(key) or existing[key] == defaults.get(key):
            raise RuntimeError(f"Missing or unsafe server secret: {key}")
    if settings.get("AUTH_REFRESH_COOKIE_SECURE") != "true":
        raise RuntimeError("HTTPS requires AUTH_REFRESH_COOKIE_SECURE=true")
    if settings.get("OLLAMA_MODEL") != "qwen2.5:7b":
        raise RuntimeError("This deployment requires qwen2.5:7b")
    for prefix in ("AUTH", "CLIENT", "BANK"):
        settings[prefix + "_DATABASE_URL"] = (
            f"postgres://{prefix.lower()}_user:{settings[prefix + '_DB_PASSWORD']}@postgres:5432/{prefix.lower()}_db?sslmode=disable")
    atomic(SHARED / "runtime.env", "".join(f"{key}={value}\n" for key, value in settings.items()))


def prepare(release, tag):
    server = release / ".server"
    server.mkdir(mode=0o700)
    apply_patches(release, (SHARED / "qwen-patches").glob("*.patch"))
    merge_env(release)
    overlay = (INSTALL / "compose.server.yml").read_text()
    overlay = overlay.replace("${RELEASE_DIR}", str(release)).replace("${RELEASE_TAG}", tag)
    atomic(server / "compose.yml", overlay)
    for name in ("web", "gateway"):
        source = "web.conf" if name == "web" else "nginx.conf"
        content = (release / "deploy/nginx" / source).read_text()
        content = content.replace("proxy_read_timeout 30s;", "proxy_read_timeout 65s;")
        content = content.replace("proxy_send_timeout 30s;", "proxy_send_timeout 65s;")
        if name == "web":
            # Only edge can reach this private web service. Use its observed client IP.
            content = content.replace("proxy_set_header X-Real-IP $remote_addr;",
                                      "proxy_set_header X-Real-IP $http_x_real_ip;")
            content = content.replace("proxy_set_header X-Forwarded-Proto $scheme;",
                                      "proxy_set_header X-Forwarded-Proto https;")
        atomic(server / (name + ".conf"), content, 0o644)
    wrapper = ("#!/bin/sh\nset -eu\nexec docker compose --env-file " + str(SHARED / "runtime.env")
               + " -p smart-credit -f " + str(release / "deploy/docker-compose.yml")
               + " -f " + str(server / "compose.yml") + ' "$@"\n')
    atomic(server / "compose", wrapper, 0o700)
    return server / "compose"


def backup(old_compose, tag):
    destination = ROOT / "backups" / (time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()) + "-" + tag)
    destination.mkdir(mode=0o700)
    for database in ("auth_db", "client_db", "bank_db"):
        with (destination / (database + ".dump")).open("wb") as stream:
            result = subprocess.run([str(old_compose), "exec", "-T", "postgres", "pg_dump", "-U", "postgres",
                                     "-Fc", database], stdout=stream, stderr=subprocess.PIPE, timeout=120)
        if result.returncode:
            raise RuntimeError("Database backup failed")
        # Check that every archive has a readable catalog before touching migrations.
        with (destination / (database + ".dump")).open("rb") as stream:
            subprocess.run([str(old_compose), "exec", "-T", "postgres", "pg_restore", "--list"],
                           stdin=stream, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                           check=True, timeout=60)
    shutil.copy2(SHARED / "runtime.env", destination / "runtime.env")
    return str(destination)


def activate(release):
    temporary = ROOT / "current.next"
    temporary.unlink(missing_ok=True)
    temporary.symlink_to(release)
    temporary.replace(ROOT / "current")
    atomic(ROOT / "compose", '#!/bin/sh\nset -eu\nexec /opt/smart-credit/current/.server/compose "$@"\n', 0o700)


def deploy(archive_source, sha):
    global logfile, settings
    os.umask(0o077)
    lock = (ROOT / "deploy.lock").open("w")
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        raise RuntimeError("Another deployment is running; retry after it completes")
    settings = read_env(SHARED / "runtime.env")
    logs = ROOT / "deploy-logs"
    logs.mkdir(exist_ok=True, mode=0o700)
    logfile = logs / (time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()) + ".log")
    os.environ["COMPOSE_PARALLEL_LIMIT"] = "2"
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise RuntimeError("Invalid source revision")
    digest = hashlib.sha256()
    for path in sorted((SHARED / "qwen-patches").glob("*.patch")):
        digest.update(path.read_bytes())
    patch_id = digest.hexdigest()[:8]
    target = sha + ":" + patch_id
    state = json.loads(STATE.read_text()) if STATE.exists() else {}
    if state.get("active_target") == target:
        phase("main unchanged: " + sha[:12])
        return
    if state.get("failed_target") == target and "--retry" not in sys.argv:
        phase("Failed revision held; waiting for new main or manual --retry: " + sha[:12])
        return
    release = ROOT / "releases" / ("main-" + sha[:12] + "-" + str(int(time.time())))
    release.mkdir(mode=0o700)
    previous = (ROOT / "current").resolve()
    previous_compose = previous / ".server/compose"
    if not previous_compose.exists():
        previous_compose = ROOT / "compose-before-autodeploy"
        shutil.copy2(ROOT / "compose", previous_compose)
    changed = False
    state.update(last_attempt=sha, last_attempt_at=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                 log=str(logfile), status="deploying")
    atomic(STATE, json.dumps(state, indent=2) + "\n")
    try:
        phase("Preparing main " + sha[:12])
        extract_source(archive_source, release, sha)
        tag = sha[:12] + "-" + patch_id
        compose = prepare(release, tag)
        run([compose, "config", "--quiet"])
        if shutil.disk_usage(ROOT).free < 12 * 1024**3:
            raise RuntimeError("Less than 12 GiB disk free; deployment held")
        phase("Building release images; existing services remain available")
        run([compose, "--profile", "tools", "build", *SERVICES])
        phase("Checking Python imports as the images' non-root users")
        run(["docker", "run", "--rm", "--network", "none", "--entrypoint", ".venv/bin/python",
             "smart-credit-scoring:" + tag, "-c", "import smart_credit_scoring.grpc_service"])
        run(["docker", "run", "--rm", "--network", "none", "--entrypoint", ".venv/bin/python",
             "smart-credit-assistant:" + tag, "-c", "import server; import help.service"])
        phase("Backing up databases and applying forward migrations")
        state["backup"] = backup(previous_compose, tag)
        for name in ("auth", "client", "bank"):
            run([compose, "run", "--rm", "--no-deps", "-T", "ops", "sh", "-ec",
                 f'exec goose -dir /migrations/{name} postgres "${name.upper()}_DATABASE_URL" up'])
        phase("Switching services and waiting for health checks")
        changed = True
        run([compose, "up", "-d", "--no-build", "--wait", "--wait-timeout", "180"], timeout=300)
        run([compose, "exec", "-T", "edge", "nginx", "-t"])
        run(["python3", INSTALL / "smoke.py"], timeout=180)
        run([compose, "exec", "-T", "ollama", "ollama", "show", "qwen2.5:7b"], timeout=30)
        activate(release)
        state.update(status="healthy", active_sha=sha, active_target=target, active_release=str(release),
                     previous_release=str(previous), failed_target=None, error=None, rollback=None,
                     completed_at=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()))
        atomic(STATE, json.dumps(state, indent=2) + "\n")
        phase("Deployed and verified main " + sha[:12])
    except Exception as error:
        if changed:
            phase("Release failed checks; restoring previous service images (no database downgrade)")
            try:
                run([previous_compose, "up", "-d", "--no-build", "--wait", "--wait-timeout", "180"], timeout=300)
                state["rollback"] = "previous images healthy; forward migrations retained"
            except Exception:
                state["rollback"] = "FAILED: operator intervention required"
        state.update(status="failed", failed_target=target, error=redact(str(error)))
        atomic(STATE, json.dumps(state, indent=2) + "\n")
        raise


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--sha", required=True)
    parser.add_argument("--retry", action="store_true")
    arguments = parser.parse_args()
    try:
        deploy(arguments.archive, arguments.sha)
    except Exception as error:
        print("Deploy failed: " + redact(str(error)), flush=True)
        raise SystemExit(1)
