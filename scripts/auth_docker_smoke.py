"""Run the existing auth/client smoke test in a disposable Compose project."""

import os
from pathlib import Path
import subprocess
import tempfile
from urllib.parse import quote
import uuid


ROOT = Path(__file__).resolve().parents[1]


def main():
    project = f"smart-credit-auth-smoke-{uuid.uuid4().hex[:12]}"
    # Use only demonstration settings; a caller's .env must not override them.
    example = (ROOT / ".env.example").read_text(encoding="utf-8")
    settings = dict(
        line.split("=", 1)
        for line in example.splitlines()
        if line and not line.startswith("#") and "=" in line
    )
    settings["COMPOSE_PROJECT_NAME"] = project
    env = os.environ.copy()
    env.update(settings)
    for key in ("COMPOSE_FILE", "COMPOSE_PROFILES", "AUTH_DOCKER_TEST_URL"):
        env.pop(key, None)

    def run(args, capture=False, check=True):
        return subprocess.run(
            args, cwd=ROOT, env=env, check=check, text=True,
            stdout=subprocess.PIPE if capture else None,
        )

    with tempfile.TemporaryDirectory(prefix="smart-credit-auth-smoke-") as directory:
        temporary = Path(directory)
        env_file = temporary / "smoke.env"
        env_file.write_text(
            "".join(f"{key}={value}\n" for key, value in settings.items()),
            encoding="utf-8",
        )
        override = temporary / "compose.override.yml"
        # Dynamic host ports keep the smoke test independent of running stacks.
        override.write_text(
            "services:\n"
            "  postgres:\n    ports: !override [\"127.0.0.1::5432\"]\n"
            "  kafka:\n    ports: !reset []\n"
            "  auth:\n    ports: !reset []\n"
            "  client-service:\n    ports: !reset []\n"
            "  swagger-ui:\n    ports: !reset []\n"
            "  gateway:\n    ports: !override [\"127.0.0.1::8080\"]\n",
            encoding="utf-8",
        )
        compose = [
            "docker", "compose", "--env-file", str(env_file),
            "--project-name", project,
            "-f", str(ROOT / "deploy/docker-compose.yml"), "-f", str(override),
        ]

        def published_port(service, port):
            address = run(compose + ["port", service, str(port)], capture=True).stdout.strip()
            return int(address.rsplit(":", 1)[1])

        def dsn(role, password, database, port):
            return f"postgres://{role}:{quote(password, safe='')}@127.0.0.1:{port}/{database}?sslmode=disable"

        print(f"Starting isolated smoke project: {project}", flush=True)
        try:
            run(compose + ["config", "--quiet"])
            run(compose + ["build", "auth", "client-service"])
            run(compose + [
                "run", "--rm", "--no-deps", "--entrypoint", "/usr/local/bin/keygen",
                "auth", "-out", "/var/lib/smart-credit/auth/private.pem",
            ])
            run(compose + ["up", "-d", "--wait", "postgres", "kafka", "swagger-ui"])
            postgres_port = published_port("postgres", 5432)
            auth_dsn = dsn("auth_user", settings["AUTH_DB_PASSWORD"], "auth_db", postgres_port)
            client_dsn = dsn("client_user", settings["CLIENT_DB_PASSWORD"], "client_db", postgres_port)
            run([
                "make", f"ENV_FILE={env_file}", f"AUTH_DATABASE_URL={auth_dsn}",
                f"CLIENT_DATABASE_URL={client_dsn}", "auth-db-up", "client-db-up",
            ])
            for topic in ("client.events", "bank.events"):
                run(compose + [
                    "exec", "-T", "kafka", "/opt/kafka/bin/kafka-topics.sh",
                    "--bootstrap-server", "kafka:19092", "--create", "--if-not-exists",
                    "--topic", topic, "--partitions", "1", "--replication-factor", "1",
                ])
            run(compose + ["up", "-d", "--wait", "auth", "client-service", "gateway"])
            env.update({
                "AUTH_DOCKER_TEST_URL": f"http://127.0.0.1:{published_port('gateway', 8080)}",
                "AUTH_DOCKER_TEST_PROJECT": project,
                "AUTH_DOCKER_TEST_ENV": str(env_file),
                "AUTH_DOCKER_TEST_OVERRIDE": str(override),
                "AUTH_DOCKER_TEST_DATABASE_URL": auth_dsn,
                "CLIENT_DOCKER_TEST_DATABASE_URL": client_dsn,
            })
            print("Running auth/client Docker integration test", flush=True)
            run([
                "go", "test", "-race", "-count=1", "-timeout=4m",
                "-run", "^TestDockerAuthClientIntegration$",
                "./services/auth/internal/container",
            ])
        except (subprocess.CalledProcessError, KeyboardInterrupt):
            run(compose + [
                "logs", "--no-color", "--tail=100",
                "postgres", "kafka", "auth", "client-service", "gateway",
            ], check=False)
            raise
        finally:
            print(f"Removing isolated smoke project: {project}", flush=True)
            run(compose + ["down", "--volumes", "--remove-orphans"])


if __name__ == "__main__":
    main()
