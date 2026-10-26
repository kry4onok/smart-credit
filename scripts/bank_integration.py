"""Shared HTTP assertions for the seeded Bank CI scenarios."""

import json
import time
import urllib.error
import urllib.request


def verify_bank_integration(base_url, token):
    cases = {
        6: ("MANUAL_REVIEW", None, True, False),
        7: ("MANUAL_REVIEW", "BUREAU_TIMEOUT", False, False),
    }
    deadline = time.monotonic() + 120
    while time.monotonic() < deadline:
        pending = []
        for number, (status, bureau_error, has_score, has_offer) in cases.items():
            case_id = f"10000000-0000-4000-8000-{number:012d}"
            url = f"{base_url}/backoffice/applications/{case_id}"
            try:
                request = urllib.request.Request(
                    url, headers={"Authorization": f"Bearer {token}"}
                )
                with urllib.request.urlopen(request, timeout=5) as response:
                    case = json.load(response)
            except (OSError, urllib.error.URLError, ValueError) as error:
                pending.append(f"{case_id}: {error}")
                continue
            checks = case.get("checks") or {}
            actual = (
                case.get("status"),
                checks.get("bureau_error"),
                bool(case.get("score")),
                bool(case.get("offer")),
            )
            expected = (status, bureau_error, has_score, has_offer)
            if actual != expected:
                pending.append(f"{case_id}: got {actual}, want {expected}")
            if number == 6 and actual == expected:
                request = urllib.request.Request(
                    url + "/assessment", headers={"Authorization": f"Bearer {token}"}
                )
                with urllib.request.urlopen(request, timeout=5) as response:
                    snapshot = json.load(response)
                assessment = snapshot.get("assessment") or {}
                if (
                    snapshot.get("case_status") != "MANUAL_REVIEW"
                    or snapshot.get("freshness") != "CURRENT"
                    or assessment.get("status") != "NO_AFFORDABLE_OPTION"
                    or not assessment.get("assessment_id")
                    or assessment.get("input_revision")
                    != snapshot.get("input_revision")
                    or assessment.get("assessment_generation")
                    != snapshot.get("assessment_generation")
                    or assessment.get("candidates") != []
                    or not assessment.get("original_failure_codes")
                    or (assessment.get("risk") or {}).get("status") != "ASSESSED"
                ):
                    pending.append(
                        f"{case_id}: missing or inconsistent assessment: {snapshot}"
                    )
        if not pending:
            print(
                "Bank integration passed: assessment review and bureau timeout cases",
                flush=True,
            )
            break
        time.sleep(2)
    else:
        raise SystemExit("Bank integration failed: " + "; ".join(pending))
