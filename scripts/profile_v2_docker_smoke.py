"""Exercise the v2 profile across auth, client, Kafka, and bank in disposable Compose."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
from urllib.error import HTTPError
from urllib.parse import quote
from urllib.request import Request, urlopen
import uuid


ROOT = Path(__file__).resolve().parents[1]
PRODUCT_ID = "10000000-0000-4000-8000-000000000021"


def main():
    settings = dict(line.split("=", 1) for line in (ROOT / ".env.example").read_text().splitlines()
                    if line and not line.startswith("#") and "=" in line)
    project = "smart-credit-v2-smoke-" + uuid.uuid4().hex[:12]
    settings["COMPOSE_PROJECT_NAME"] = project
    secret = "v2-smoke-" + uuid.uuid4().hex
    settings.update(AUTH_DEV_ANALYST_PASSWORD=secret, AUTH_DEV_ADMIN_PASSWORD=secret,
                    AUTH_DEV_CLIENT_PASSWORD=secret)
    env = os.environ.copy()
    env.update(settings)
    env["GOCACHE"] = "/private/tmp/smart-credit-go-cache"
    for key in ("COMPOSE_FILE", "COMPOSE_PROFILES"):
        env.pop(key, None)

    def run(args, input_data=None):
        result = subprocess.run(args, cwd=ROOT, env=env, input=input_data, text=True,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode:
            raise RuntimeError(f"isolated smoke command failed ({result.returncode}): {args[0]} {args[-1]}")
        return result.stdout.strip()

    with tempfile.TemporaryDirectory(prefix="smart-credit-v2-smoke-") as directory:
        temp = Path(directory)
        env_file = temp / "smoke.env"
        env_file.write_text("".join(f"{key}={value}\n" for key, value in settings.items()))
        nginx = temp / "nginx.conf"
        # The gateway already routes the bank through the Compose network.
        nginx.write_text((ROOT / "deploy/nginx/nginx.conf").read_text())
        override = temp / "override.yml"
        override.write_text("services:\n"
                            "  postgres:\n    ports: !override [\"127.0.0.1::5432\"]\n"
                            "  gateway:\n    ports: !override [\"127.0.0.1::8080\"]\n"
                            f"    volumes: !override [\"{nginx}:/etc/nginx/nginx.conf:ro\","
                            f" \"{ROOT / 'api'}:/usr/share/nginx/html/api:ro\"]\n"
                            + "".join(f"  {name}:\n    ports: !reset []\n" for name in
                                      ("kafka", "auth", "client-service", "bank-service", "swagger-ui",
                                       "scoring", "bureau-mocks", "assistant")))
        compose = ["docker", "compose", "--env-file", str(env_file), "--project-name", project,
                   "-f", str(ROOT / "deploy/docker-compose.yml"), "-f", str(override)]
        try:
            print("Building isolated services", flush=True)
            run(compose + ["config", "--quiet"])
            run(compose + ["build", "auth", "client-service", "bank-service", "scoring", "bureau-mocks", "assistant"])
            run(compose + ["run", "--rm", "--no-deps", "--entrypoint", "/usr/local/bin/keygen",
                           "auth", "-out", "/var/lib/smart-credit/auth/private.pem"])
            print("Starting PostgreSQL and Kafka", flush=True)
            run(compose + ["up", "-d", "--wait", "postgres", "kafka", "swagger-ui"])
            postgres_port = int(run(compose + ["port", "postgres", "5432"]).rsplit(":", 1)[1])
            def dsn(role, password, database):
                return f"postgres://{role}:{quote(password, safe='')}@127.0.0.1:{postgres_port}/{database}?sslmode=disable"
            db_urls = {"AUTH_DATABASE_URL": dsn("auth_user", settings["AUTH_DB_PASSWORD"], "auth_db"),
                       "CLIENT_DATABASE_URL": dsn("client_user", settings["CLIENT_DB_PASSWORD"], "client_db"),
                       "BANK_DATABASE_URL": dsn("bank_user", settings["BANK_DB_PASSWORD"], "bank_db")}
            env.update(db_urls)
            print("Applying migrations and isolated fixtures", flush=True)
            run(["make", f"ENV_FILE={env_file}", *[f"{key}={value}" for key, value in db_urls.items()],
                 "auth-db-up", "client-db-up", "bank-db-up", "auth-dev-seed"])
            seed_sql = """
                INSERT INTO products (id,version,code,name,description,purpose_tags,required_document_types,
                  min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,indicative_rate_bps,
                  is_active,requirements_policy_version) VALUES
                ('10000000-0000-4000-8000-000000000021',1,'SME_PROFILE_V2','Демо: профиль ИП/ООО',
                 'Изолированный тест',ARRAY['WORKING_CAPITAL'],ARRAY[]::text[],10000000,500000000,6,36,
                 1800,true,'sme-profile-v1');
                INSERT INTO product_rules (product_id,product_version,min_company_age_months,
                  max_amount_to_revenue_bps,max_overdue_days,rate_grade_a_bps,rate_grade_b_bps,offer_ttl_hours)
                VALUES ('10000000-0000-4000-8000-000000000021',1,12,5000,30,1600,1800,72);
            """
            run(compose + ["exec", "-T", "postgres", "psql", "-X", "-v", "ON_ERROR_STOP=1",
                           "-U", "postgres", "-d", "bank_db"], seed_sql)
            for topic in ("client.events", "bank.events"):
                run(compose + ["exec", "-T", "kafka", "/opt/kafka/bin/kafka-topics.sh", "--bootstrap-server",
                               "kafka:19092", "--create", "--if-not-exists", "--topic", topic,
                               "--partitions", "1", "--replication-factor", "1"])
            print("Starting auth, client, and bank", flush=True)
            run(compose + ["up", "-d", "--wait", "auth", "client-service", "bank-service", "scoring",
                           "bureau-mocks", "assistant", "gateway"])
            port = int(run(compose + ["port", "gateway", "8080"]).rsplit(":", 1)[1])
            base = f"http://127.0.0.1:{port}"

            def api(method, path, token=None, body=None, headers=None, want=200):
                data = None if body is None else json.dumps(body).encode()
                h = dict(headers or {})
                if token:
                    h["Authorization"] = "Bearer " + token
                if data is not None and "Content-Type" not in h:
                    h["Content-Type"] = "application/json"
                for attempt in range(10):
                    req = Request(base + path, data=data, headers=h, method=method)
                    try:
                        response = urlopen(req, timeout=15)
                    except HTTPError as error:
                        response = error
                    raw = response.read()
                    if response.status not in (502, 503, 504) or attempt == 9:
                        break
                    time.sleep(1)
                if response.status != want:
                    raise AssertionError(f"{method} {path}: HTTP {response.status}, expected {want}: {raw[:700]!r}")
                return json.loads(raw) if raw else None

            def login(email):
                return api("POST", "/api/auth/login", body={"email": email, "password": secret})["access_token"]

            client = login("client@smart-credit.test")
            analyst = login("analyst@smart-credit.test")
            profile = json.loads((ROOT / "seed/profile-requirements/company-llc-usn.json").read_text())
            profile["inn"] = "7707083893"
            api("PUT", "/api/client/company", client, profile)
            deadline = time.monotonic() + 90
            while True:
                products = api("GET", "/api/client/products", client)
                if PRODUCT_ID in json.dumps(products):
                    break
                if time.monotonic() > deadline:
                    raise AssertionError("v2 product was not published through Kafka")
                time.sleep(1)
            draft = api("POST", "/api/client/applications", client, {"product_id": PRODUCT_ID},
                        {"Idempotency-Key": str(uuid.uuid4())}, 201)
            application_id = draft["id"]
            form = {"amount": {"amount_kopecks": 100000000, "currency": "RUB"},
                    "term_months": 12, "purpose": "WORKING_CAPITAL"}
            updated = api("PATCH", f"/api/client/applications/{application_id}", client, form,
                          {"If-Match": f'"{draft["version"]}"', "Content-Type": "application/merge-patch+json"})
            if not updated.get("requirements_plan", {}).get("can_submit"):
                raise AssertionError("complete v2 draft cannot submit")
            api("POST", f"/api/client/applications/{application_id}/submit", client, {},
                {"Idempotency-Key": str(uuid.uuid4())})
            deadline = time.monotonic() + 90
            while True:
                app = api("GET", f"/api/client/applications/{application_id}", client)
                if app["status"] == "NEEDS_INFO" and app.get("info_request", {}).get("requirements"):
                    break
                if time.monotonic() > deadline:
                    raise AssertionError("bank NEEDS_INFO did not reach client through Kafka")
                time.sleep(1)
            bank_case = api("GET", f"/api/backoffice/applications/{application_id}", analyst)
            if bank_case.get("status") != "NEEDS_INFO" or not bank_case.get("requirements_plan"):
                raise AssertionError("bank case lost requirements plan")
            document_ids = []
            for requirement in app["info_request"]["requirements"]:
                if requirement["kind"] != "DOCUMENT":
                    raise AssertionError("unexpected field in complete submitted profile")
                boundary = "v2smoke" + uuid.uuid4().hex
                fields = {"document_type": requirement["accepted_document_types"][0],
                          "requirement_id": requirement["id"],
                          "coverage_from": requirement["period"]["from"],
                          "coverage_to": requirement["period"]["to"]}
                parts = []
                for key, value in fields.items():
                    parts.append(f"--{boundary}\r\nContent-Disposition: form-data; name=\"{key}\"\r\n\r\n{value}\r\n".encode())
                parts.append(f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"demo.txt\"\r\nContent-Type: text/plain\r\n\r\nДемо документ\r\n".encode())
                parts.append(f"--{boundary}--\r\n".encode())
                req = Request(base + f"/api/client/applications/{application_id}/documents",
                              data=b"".join(parts), method="POST",
                              headers={"Authorization": "Bearer " + client,
                                       "Idempotency-Key": str(uuid.uuid4()),
                                       "Content-Type": f"multipart/form-data; boundary={boundary}"})
                try:
                    response = urlopen(req, timeout=15)
                except HTTPError as error:
                    response = error
                raw = response.read()
                if response.status != 201:
                    raise AssertionError(f"document upload: HTTP {response.status}: {raw[:700]!r}")
                document_ids.append(json.loads(raw)["id"])
            api("POST", f"/api/client/applications/{application_id}/info", client,
                {"request_id": app["info_request"]["id"], "document_ids": document_ids},
                {"Idempotency-Key": str(uuid.uuid4())}, 202)
            deadline = time.monotonic() + 90
            while True:
                app = api("GET", f"/api/client/applications/{application_id}", client)
                if app["status"] not in ("NEEDS_INFO", "SUBMITTED", "UNDER_REVIEW"):
                    break
                bank_case = api("GET", f"/api/backoffice/applications/{application_id}", analyst)
                if bank_case.get("status") != "NEEDS_INFO" and app["status"] == "UNDER_REVIEW":
                    break
                if time.monotonic() > deadline:
                    raise AssertionError("InfoProvided did not advance bank case")
                time.sleep(1)
            if bank_case.get("status") == "NEEDS_INFO":
                raise AssertionError("bank kept a satisfied request open")
            deadline = time.monotonic() + 120
            while bank_case.get("status") not in ("OFFER", "COUNTER_OFFER", "MANUAL_REVIEW", "REJECTED"):
                if time.monotonic() > deadline:
                    raise AssertionError(f"decision worker did not finish v2 case: {bank_case.get('status')}")
                time.sleep(1)
                bank_case = api("GET", f"/api/backoffice/applications/{application_id}", analyst)
                if bank_case.get("status") == "NEEDS_INFO":
                    raise AssertionError("bank reopened a satisfied request")
            print(f"E2E v2 catalog → submit → NEEDS_INFO → InfoProvided → {bank_case['status']} passed ({application_id})", flush=True)
        except Exception:
            diagnostics = subprocess.run(compose + ["logs", "--no-color", "--tail=20",
                                                  "gateway", "client-service", "bank-service"],
                                         cwd=ROOT, env=env, text=True, stdout=subprocess.PIPE,
                                         stderr=subprocess.PIPE)
            print(diagnostics.stdout[-5000:], flush=True)
            raise
        finally:
            print("Removing isolated Compose project", flush=True)
            subprocess.run(compose + ["down", "--volumes", "--remove-orphans"], cwd=ROOT, env=env,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)


if __name__ == "__main__":
    main()
