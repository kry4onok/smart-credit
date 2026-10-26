"""Exercise cash-flow assessment, Help, recheck, offers and demo disbursement in disposable Compose."""

import argparse
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


from bank_integration import verify_bank_integration


ROOT = Path(__file__).resolve().parents[1]
PRODUCT_ID = "10000000-0000-4000-8000-000000000022"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--images-from", help="Reuse images from a previous smart-credit-assessment-smoke-* project")
    args = parser.parse_args()
    if args.images_from and not args.images_from.startswith("smart-credit-assessment-smoke-"):
        parser.error("--images-from must identify an assessment smoke project")
    settings = dict(line.split("=", 1) for line in (ROOT / ".env.example").read_text().splitlines()
                    if line and not line.startswith("#") and "=" in line)
    project = "smart-credit-assessment-smoke-" + uuid.uuid4().hex[:12]
    settings["COMPOSE_PROJECT_NAME"] = project
    settings["OLLAMA_URL"] = "http://127.0.0.1:1"
    secret = "v2-smoke-" + uuid.uuid4().hex
    settings.update(AUTH_DEV_ANALYST_PASSWORD=secret, AUTH_DEV_ADMIN_PASSWORD=secret,
                    AUTH_DEV_CLIENT_PASSWORD=secret)
    env = os.environ.copy()
    env.update(settings)
    for key in ("COMPOSE_FILE", "COMPOSE_PROFILES"):
        env.pop(key, None)

    def run(args, input_data=None):
        result = subprocess.run(args, cwd=ROOT, env=env, input=input_data, text=True,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=900)
        if result.returncode:
            raise RuntimeError(f"isolated smoke command failed ({result.returncode}): {args[0]} {args[-1]}: {result.stderr[-3000:]}")
        return result.stdout.strip()

    with tempfile.TemporaryDirectory(prefix="smart-credit-assessment-smoke-") as directory:
        temp = Path(directory)
        env_file = temp / "smoke.env"
        env_file.write_text("".join(f"{key}={value}\n" for key, value in settings.items()))
        nginx = temp / "nginx.conf"
        nginx.write_text((ROOT / "deploy/nginx/nginx.conf").read_text().replace(
            "host.docker.internal:8080", "bank_backend").replace(
            "    server {\n", "    upstream bank_backend {\n        zone bank_backend 64k;\n"
            "        server bank-service:8080 resolve;\n    }\n\n    server {\n", 1))
        override = temp / "override.yml"
        override.write_text("services:\n"
                            "  postgres:\n    ports: !override [\"127.0.0.1::5432\"]\n"
                            "  gateway:\n    ports: !override [\"127.0.0.1::8080\"]\n"
                            f"    volumes: !override [\"{nginx}:/etc/nginx/nginx.conf:ro\","
                            f" \"{ROOT / 'api'}:/usr/share/nginx/html/api:ro\"]\n"
                            + "".join(f"  {name}:\n    ports: !reset []\n" + (f"    image: {args.images_from}-{name}:latest\n" if args.images_from and name not in ("kafka", "swagger-ui") else "") for name in
                                      ("kafka", "auth", "client-service", "bank-service", "swagger-ui",
                                       "scoring", "bureau-mocks", "assistant")))
        compose = ["docker", "compose", "--env-file", str(env_file), "--project-name", project,
                   "-f", str(ROOT / "deploy/docker-compose.yml"), "-f", str(override)]
        try:
            print("Using previous smoke images" if args.images_from else "Building isolated services", flush=True)
            run(compose + ["config", "--quiet"])
            if not args.images_from:
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
                ('10000000-0000-4000-8000-000000000022',1,'SME_CASHFLOW_V2','Демо: профиль ИП/ООО',
                 'Изолированный тест',ARRAY['WORKING_CAPITAL'],ARRAY[]::text[],10000000,500000000,6,36,
                 1800,true,'sme-profile-v2');
                INSERT INTO product_rules (product_id,product_version,min_company_age_months,
                  max_amount_to_revenue_bps,max_overdue_days,rate_grade_a_bps,rate_grade_b_bps,offer_ttl_hours)
                VALUES ('10000000-0000-4000-8000-000000000022',1,12,5000,30,1600,1800,168);
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
                if response.status not in (want if isinstance(want, tuple) else (want,)):
                    raise AssertionError(f"{method} {path}: HTTP {response.status}, expected {want}: {raw[:700]!r}")
                return None if response.status == 404 else (json.loads(raw) if raw else None)

            def login(email):
                return api("POST", "/api/auth/login", body={"email": email, "password": secret})["access_token"]

            client = login("client@smart-credit.test")
            analyst = login("analyst@smart-credit.test")
            def wait_case(application_id, status):
                deadline = time.monotonic() + 150
                while time.monotonic() < deadline:
                    current = api("GET", f"/api/backoffice/applications/{application_id}", analyst, want=(200, 404))
                    if current is not None and current["status"] in status:
                        return current
                    time.sleep(1)
                raise AssertionError(f"case did not reach {status}: {current['status']}")

            def wait_app(application_id, status):
                deadline = time.monotonic() + 150
                while time.monotonic() < deadline:
                    current = api("GET", f"/api/client/applications/{application_id}", client)
                    if current["status"] == status and not current.get("pending_action"):
                        return current
                    time.sleep(1)
                raise AssertionError(f"client did not reach {status}: {current['status']}")

            def new_application(profile_file, inn=None, amount=100000000):
                nonlocal client
                profile = json.loads((ROOT / "seed/profile-requirements" / profile_file).read_text())
                if inn:
                    profile["inn"] = inn
                if profile["inn"] != "7707083893":
                    organization, user = str(uuid.uuid4()), str(uuid.uuid4())
                    email = user + "@assessment.test"
                    sql = f"""INSERT INTO organizations(id,inn,name) VALUES('{organization}','{profile['inn']}','Synthetic assessment');
                        INSERT INTO users(id,email,full_name,password_hash,role,org_id,is_active)
                        SELECT '{user}','{email}','Synthetic client',password_hash,'CLIENT','{organization}',true
                        FROM users WHERE email='client@smart-credit.test';"""
                    run(compose + ["exec", "-T", "postgres", "psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "auth_db"], sql)
                    client = login(email)
                api("PUT", "/api/client/company", client, profile)
                deadline = time.monotonic() + 90
                while not any(p["id"] == PRODUCT_ID for p in api("GET", "/api/client/products", client)["items"]):
                    if time.monotonic() > deadline:
                        raise AssertionError("assessment product was not published")
                    time.sleep(1)
                draft = api("POST", "/api/client/applications", client, {"product_id": PRODUCT_ID},
                            {"Idempotency-Key": str(uuid.uuid4())}, 201)
                application_id = draft["id"]
                form = {"amount": {"amount_kopecks": amount, "currency": "RUB"},
                        "term_months": 12, "purpose": "WORKING_CAPITAL"}
                updated = api("PATCH", f"/api/client/applications/{application_id}", client, form,
                              {"If-Match": f'"{draft["version"]}"', "Content-Type": "application/merge-patch+json"})
                saved = api("PUT", f"/api/client/applications/{application_id}/assessment-input", client,
                            {"schema_version": 1, "preferences": {"minimum_useful_amount_kopecks": 100000000,
                             "priority": "KEEP_AMOUNT"}}, {"If-Match": f'"{updated["version"]}"'})
                api("PUT", f"/api/client/applications/{application_id}/assessment-input", client,
                    {"schema_version": 1, "preferences": {}}, {"If-Match": f'"{updated["version"]}"'}, 409)
                assert saved["version"] > updated["version"]
                api("POST", f"/api/client/applications/{application_id}/submit", client, {},
                    {"Idempotency-Key": str(uuid.uuid4())})
                return application_id

            print("LLC: external facts before repeated documents; alternatives and Help fallback", flush=True)
            application_id = new_application("company-llc-usn.json", "7707083893", 500000000)
            case = wait_case(application_id, {"MANUAL_REVIEW", "NEEDS_INFO", "REJECTED"})
            snapshot = api("GET", f"/api/backoffice/applications/{application_id}/assessment", analyst)
            result = snapshot["assessment"]
            assert case["status"] == "MANUAL_REVIEW" and result["status"] == "READY_FOR_CONFIRMATION", snapshot
            assert result["candidates"] and any(c["uncovered_need_kopecks"] > 0 for c in result["candidates"])
            assert all(m["quality"] == "PROJECTED" for m in result["financial"]["base"])
            assert snapshot["baseline_v1"] is not None
            deadline = time.monotonic() + 60
            while True:
                projected = api("GET", f"/api/client/applications/{application_id}/assessment", client)
                if projected["assessment"] and projected["assessment"]["assessment_id"] == result["assessment_id"] and projected["freshness"] == "CURRENT":
                    break
                if time.monotonic() > deadline:
                    raise AssertionError("completed assessment did not reach client projection")
                time.sleep(1)
            context_path = f"/api/client/applications/{application_id}/help"
            descriptor = api("GET", context_path, client)
            help_body = {"context_id": descriptor["context_id"], "source_revision": descriptor["source_revision"],
                         "catalog_version": descriptor["catalog_version"], "mode": "NEXT_STEP", "detail": "CONCISE", "target_ids": []}
            explanation = api("POST", context_path, client, help_body)
            assert explanation["source"] == "RESPONSE_SOURCE_TEMPLATE" and explanation["blocks"]
            api("POST", context_path, client, dict(help_body, catalog_version="unknown"), want=409)
            api("GET", f"/api/backoffice/applications/{application_id}/assessment", client, want=403)

            print("Recheck: unchanged input revision, new generation, idempotent replay", flush=True)
            case = api("POST", f"/api/backoffice/applications/{application_id}/take", analyst, {},
                       {"If-Match": f'"{case["version"]}"', "Idempotency-Key": str(uuid.uuid4())})
            recheck_headers = {"If-Match": f'"{case["version"]}"', "Idempotency-Key": str(uuid.uuid4())}
            first = api("POST", f"/api/backoffice/applications/{application_id}/recheck", analyst, {}, recheck_headers, 202)
            repeated = api("POST", f"/api/backoffice/applications/{application_id}/recheck", analyst, {}, recheck_headers, 202)
            assert repeated == first and first["assessment_generation"] == snapshot["assessment_generation"] + 1
            case = wait_case(application_id, {"MANUAL_REVIEW"})
            current = api("GET", f"/api/backoffice/applications/{application_id}/assessment", analyst)
            assert current["input_revision"] == snapshot["input_revision"]
            assert current["assessment"]["assessment_id"] != result["assessment_id"]
            option = current["assessment"]["candidates"][0]
            terms = option["schedule"]
            offer = {"product_id": option["product_id"], "amount": {"amount_kopecks": terms["amount_kopecks"], "currency": "RUB"},
                     "term_months": terms["term_months"], "rate_bps": terms["rate_bps"]}
            decision = {"outcome": "COUNTER", "comment": "Synthetic cash-flow smoke", "public_reason": "Предлагаем доступный вариант по денежному потоку", "offer": offer}
            bad = dict(decision, offer=dict(offer, rate_bps=1))
            api("POST", f"/api/backoffice/applications/{application_id}/decision", analyst, bad,
                {"If-Match": f'"{case["version"]}"', "Idempotency-Key": str(uuid.uuid4())}, 422)
            api("POST", f"/api/backoffice/applications/{application_id}/decision", analyst, decision,
                {"If-Match": f'"{case["version"]}"', "Idempotency-Key": str(uuid.uuid4())})
            app = wait_app(application_id, "OFFER_AVAILABLE")
            public = api("GET", f"/api/client/applications/{application_id}/assessment", client)
            assert public["assessment"] and "risk" not in public["assessment"] and "checks" not in public["assessment"]
            api("POST", context_path, client, help_body, want=409)
            offer_id = app["offer"]["id"]
            api("POST", f"/api/client/applications/{application_id}/offer/accept", client, {"offer_id": offer_id},
                {"Idempotency-Key": str(uuid.uuid4())}, 202)
            wait_app(application_id, "FORMALIZING")
            sign_headers = {"Idempotency-Key": str(uuid.uuid4())}
            api("POST", f"/api/client/applications/{application_id}/sign", client, {"offer_id": offer_id}, sign_headers, 202)
            api("POST", f"/api/client/applications/{application_id}/sign", client, {"offer_id": offer_id}, sign_headers, 202)
            wait_app(application_id, "DISBURSED")
            final = api("GET", f"/api/client/applications/{application_id}/assessment", client)
            assert final["assessment"]["assessment_id"] == current["assessment"]["assessment_id"]
            count = run(compose + ["exec", "-T", "postgres", "psql", "-X", "-At", "-U", "postgres", "-d", "bank_db", "-c",
                                   f"SELECT count(*) FROM audit_log WHERE action='DEMO_DISBURSED' AND case_id='{application_id}'"])
            assert count == "1", count
            print("Confirmation → acceptance → repeated signing → exactly one demo disbursement passed", flush=True)

            print("IP PSN: request only missing patent after source collection", flush=True)
            ip_id = new_application("company-ip-psn.json")
            api("GET", f"/api/client/applications/{application_id}/assessment", client, want=404)
            api("GET", f"/api/client/applications/{application_id}/help", client, want=404)
            wait_case(ip_id, {"NEEDS_INFO"})
            app = wait_app(ip_id, "NEEDS_INFO")
            requirements = app["info_request"]["requirements"]
            assert requirements and all(q["code"] == "PATENT_RIGHT" for q in requirements), requirements
            document_ids = []
            for requirement in requirements:
                boundary = "assessment" + uuid.uuid4().hex
                fields = {"document_type": requirement["accepted_document_types"][0], "requirement_id": requirement["id"],
                          "coverage_from": requirement["period"]["from"], "coverage_to": requirement["period"]["to"]}
                parts = [f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"\r\n\r\n{value}\r\n'.encode() for key, value in fields.items()]
                parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="demo.txt"\r\nContent-Type: text/plain\r\n\r\nSynthetic patent\r\n--{boundary}--\r\n'.encode())
                req = Request(base + f"/api/client/applications/{ip_id}/documents", data=b"".join(parts), method="POST",
                              headers={"Authorization": "Bearer " + client, "Idempotency-Key": str(uuid.uuid4()), "Content-Type": f"multipart/form-data; boundary={boundary}"})
                with urlopen(req, timeout=15) as response:
                    assert response.status == 201
                    document_ids.append(json.loads(response.read())["id"])
            api("POST", f"/api/client/applications/{ip_id}/info", client,
                {"request_id": app["info_request"]["id"], "document_ids": document_ids},
                {"Idempotency-Key": str(uuid.uuid4())}, 202)
            wait_case(ip_id, {"MANUAL_REVIEW"})
            assert api("GET", f"/api/backoffice/applications/{ip_id}/assessment", analyst)["assessment"]["status"] == "READY_FOR_CONFIRMATION"
            print("LLC liquidation: explicit admission rejection", flush=True)
            rejected_id = new_application("company-llc-usn.json", "1111111117")
            wait_app(rejected_id, "REJECTED")
            rejected = api("GET", f"/api/backoffice/applications/{rejected_id}/assessment", analyst)["assessment"]
            assert rejected["status"] == "REJECT" and "COMPANY_LIQUIDATION" in rejected["checks"]["codes"]
            print("Bank CI: seeded enrichment and assessment routing", flush=True)
            run(compose + ["exec", "-T", "postgres", "psql", "-X", "-v", "ON_ERROR_STOP=1",
                           "-U", "postgres", "-d", "bank_db"],
                (ROOT / "services/bank-service/seeds/demo.sql").read_text())
            verify_bank_integration(base + "/api", analyst)
            print("All assessment E2E scenarios passed", flush=True)
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
