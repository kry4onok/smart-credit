#!/usr/bin/env python3
"""Exercise the same auth/session endpoints used by the browser, without logging secrets."""
import http.cookiejar
import json
import os
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import HTTPCookieProcessor, Request, build_opener

ROOT = Path(os.environ.get("DEPLOY_ROOT", "/opt/smart-credit"))
BASE = os.environ.get("PUBLIC_URL", "https://185.149.147.246")


def run():
    settings = dict(line.split("=", 1) for line in (ROOT / "shared/runtime.env").read_text().splitlines()
                    if line and not line.startswith("#") and "=" in line)
    accounts = [("client", "CLIENT"), ("analyst", "ANALYST"), ("admin", "ADMIN"),
                ("ip", "CLIENT"), ("closing", "CLIENT")]
    for name, role in accounts:
        jar = http.cookiejar.CookieJar()
        opener = build_opener(HTTPCookieProcessor(jar))

        def api(method, path, body=None, token=None, expected=200):
            headers = {"Content-Type": "application/json"}
            if token:
                headers["Authorization"] = "Bearer " + token
            request = Request(BASE + path, method=method, headers=headers,
                              data=None if body is None else json.dumps(body).encode())
            try:
                response = opener.open(request, timeout=30)
            except HTTPError as error:
                response = error
            if response.status != expected:
                raise RuntimeError(f"{name}: {method} {path}: HTTP {response.status}, expected {expected}")
            data = response.read()
            return json.loads(data) if data else None

        token = api("POST", "/api/auth/login", {"email": name + "@smart-credit.test",
                    "password": settings["AUTH_DEV_" + role + "_PASSWORD"]})["access_token"]
        assert api("GET", "/api/auth/me", token=token)["role"] == role
        cookies = list(jar)
        assert cookies and all(c.secure and c.has_nonstandard_attr("HttpOnly") for c in cookies)
        assert all(c.get_nonstandard_attr("SameSite", "").lower() in ("lax", "strict") for c in cookies)
        # A reload loses the in-memory JWT; the refresh cookie must restore the session.
        token = api("POST", "/api/auth/refresh")["access_token"]
        assert api("GET", "/api/auth/me", token=token)["role"] == role
        path = "/api/client/products" if role == "CLIENT" else "/api/backoffice/applications"
        api("GET", path, token=token)
        api("POST", "/api/auth/logout", expected=204)
        api("POST", "/api/auth/refresh", expected=401)
        print(f"{name}: login, role, secure cookie, refresh, protected API, logout PASS", flush=True)
    print("Authorization smoke: PASS", flush=True)


if __name__ == "__main__":
    try:
        run()
    except Exception as error:
        # Do not print request bodies, tokens, cookies or settings on failure.
        print(f"Authorization smoke: FAIL ({type(error).__name__})", flush=True)
        raise SystemExit(1)
