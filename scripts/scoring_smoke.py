#!/usr/bin/env python3
"""Verify shared contract fixtures against a real scoring gRPC endpoint."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Any

import grpc
from google.protobuf.json_format import ParseDict
from grpc_health.v1 import health_pb2, health_pb2_grpc

from proto.scoring import scoring_pb2, scoring_pb2_grpc

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_FIXTURES = REPO_ROOT / "seed/scoring/personas.json"
DEFAULT_CONFIG = REPO_ROOT / "ml/scoring/models/scorecard-v1.json"
FACTOR_NAMES = {"business_age", "profit_margin", "bureau_debt_load", "payment_discipline"}


class ContractError(ValueError):
    """The service response or fixture violates the v1 contract."""


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ContractError(message)


def load_cases(path: Path, case_name: str | None = None) -> list[dict[str, Any]]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    require(payload["schema_version"] == "scoring-personas-v1", "Unknown fixture schema")
    cases = payload["cases"]
    require(len({case["name"] for case in cases}) == len(cases), "Duplicate fixture names")
    if case_name is not None:
        cases = [case for case in cases if case["name"] == case_name]
    require(bool(cases), "No matching fixtures")
    return cases


def make_request(case: dict[str, Any]) -> scoring_pb2.ScoreRequest:
    return ParseDict(case["request"], scoring_pb2.ScoreRequest(), ignore_unknown_fields=False)


def verify_response(
    case: dict[str, Any], response: scoring_pb2.ScoreResponse, checksum: str
) -> dict[str, Any]:
    expected = case["expected"]
    expected_status = scoring_pb2.AssessmentStatus.Value(
        "ASSESSMENT_STATUS_" + expected["assessment_status"]
    )
    require(response.assessment_status == expected_status, "Assessment status mismatch")
    require(response.model_version == "monotone-scorecard-v1", "Model version mismatch")
    require(response.feature_schema_version == "borrower-features-v1", "Feature version mismatch")
    require(response.baseline_score == 500, "Baseline mismatch")
    require(
        bool(re.fullmatch(r"[0-9a-f]{64}", response.config_checksum)), "Invalid checksum format"
    )
    require(response.config_checksum == checksum, "Model artifact checksum mismatch")
    issues = [
        {"code": issue.code, "field_path": issue.field_path} for issue in response.data_issues
    ]
    require(issues == expected["data_issues"], "Data issues mismatch")
    require(
        issues == sorted(issues, key=lambda issue: (issue["field_path"], issue["code"])),
        "Data issues order mismatch",
    )

    if expected_status != scoring_pb2.ASSESSMENT_STATUS_ASSESSED:
        require(not response.HasField("score"), "Non-assessed response contains a score")
        require(
            response.grade == scoring_pb2.GRADE_UNSPECIFIED, "Non-assessed response has a grade"
        )
        require(
            not response.factor_contributions and not response.reasons,
            "Non-assessed response contains explanations",
        )
        return {"case": case["name"], "status": expected["assessment_status"]}

    require(response.HasField("score"), "Assessed response is missing score presence")
    require(response.score == expected["score"], "Score mismatch")
    require(
        response.grade == scoring_pb2.Grade.Value("GRADE_" + expected["grade"]), "Grade mismatch"
    )
    factors = list(response.factor_contributions)
    require(
        len(factors) == 4 and {factor.factor_name for factor in factors} == FACTOR_NAMES,
        "Expected exactly four unique factor groups",
    )
    require(all(factor.description.strip() for factor in factors), "Missing factor descriptions")
    actual_points = {factor.factor_name: factor.points for factor in factors}
    expected_points = {
        factor["factor_name"]: factor["points"] for factor in expected["factor_contributions"]
    }
    require(actual_points == expected_points, "Factor points mismatch")
    require(
        500 + sum(actual_points.values()) == response.score, "Contributions do not sum to score"
    )

    # Determine explanations from the independently fixed fixture points, not the model formula.
    code_by_factor = {
        "business_age": ("SHORT_HISTORY", "STABLE_BUSINESS"),
        "profit_margin": ("LOW_PROFIT_MARGIN", "HEALTHY_PROFIT_MARGIN"),
        "bureau_debt_load": ("HIGH_BUREAU_DEBT", "MANAGEABLE_BUREAU_DEBT"),
        "payment_discipline": (
            "RECENT_OVERDUE",
            "NO_OVERDUE"
            if case["request"]["features"]["overdueCount12m"] == 0
            and case["request"]["features"]["maxOverdueDays"] == 0
            else "GOOD_PAYMENT_DISCIPLINE",
        ),
    }
    expected_reasons = []
    for factor_name, points in expected_points.items():
        if points:
            code = code_by_factor[factor_name][int(points > 0)]
            direction = (
                scoring_pb2.DIRECTION_DECREASES_RISK
                if points > 0
                else scoring_pb2.DIRECTION_INCREASES_RISK
            )
            expected_reasons.append((abs(points), code, direction))
    expected_reasons.sort(key=lambda reason: (-reason[0], reason[1]))
    actual_reasons = [
        (
            reason.contribution,
            scoring_pb2.ReasonCode.Name(reason.code).removeprefix("REASON_CODE_"),
            reason.direction,
        )
        for reason in response.reasons
    ]
    require(actual_reasons == expected_reasons[:3], "Reasons, directions, or ordering mismatch")
    require(
        all(reason.description.strip() for reason in response.reasons),
        "Missing reason descriptions",
    )
    return {
        "case": case["name"],
        "status": "ASSESSED",
        "score": response.score,
        "grade": expected["grade"],
    }


def check_health(channel: grpc.Channel, timeout: float = 2.0) -> None:
    health = health_pb2_grpc.HealthStub(channel)
    for service in ("", "smartcredit.scoring.ScoringService"):
        result = health.Check(health_pb2.HealthCheckRequest(service=service), timeout=timeout)
        require(
            result.status == health_pb2.HealthCheckResponse.SERVING, "Scoring health is not SERVING"
        )


def run_cases(
    channel: grpc.Channel, cases: list[dict[str, Any]], checksum: str, timeout: float = 2.0
) -> list[dict[str, Any]]:
    stub = scoring_pb2_grpc.ScoringServiceStub(channel)
    results = []
    for case in cases:
        try:
            response = stub.Score(make_request(case), timeout=timeout)
            results.append(verify_response(case, response, checksum))
        except ContractError as error:
            raise ContractError(f"{case['name']}: {error}") from error
    return results


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", default="127.0.0.1:50051")
    parser.add_argument("--fixtures", type=Path, default=DEFAULT_FIXTURES)
    parser.add_argument(
        "--config",
        type=Path,
        default=DEFAULT_CONFIG,
        help="Exact local model artifact expected on the server",
    )
    parser.add_argument("--case", dest="case_name")
    arguments = parser.parse_args()
    try:
        cases = load_cases(arguments.fixtures, arguments.case_name)
        checksum = hashlib.sha256(arguments.config.read_bytes()).hexdigest()
        with grpc.insecure_channel(arguments.target) as channel:
            check_health(channel)
            results = run_cases(channel, cases, checksum)
        print(
            json.dumps({"passed": len(results), "results": results}, ensure_ascii=False, indent=2)
        )
        return 0
    except grpc.RpcError as error:
        print(f"Scoring RPC failed: {error.code().name}", file=sys.stderr)
    except (ContractError, OSError, KeyError, ValueError) as error:
        print(f"Scoring smoke failed: {error}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
