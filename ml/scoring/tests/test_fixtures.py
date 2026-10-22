"""Check the bank-facing examples remain internally coherent as files evolve."""

import calendar
import json
from datetime import date
from pathlib import Path
from uuid import UUID

import pytest
from google.protobuf.json_format import ParseDict

from proto.scoring import scoring_pb2

FIXTURES_PATH = Path(__file__).resolve().parents[3] / "seed/scoring/personas.json"


def full_months(registered: date, as_of: date) -> int:
    """Reference calendar convention for fixed examples, not a bank production adapter."""
    if registered > as_of:
        raise ValueError("registration is in the future")
    anniversary_day = min(registered.day, calendar.monthrange(as_of.year, as_of.month)[1])
    return (
        (as_of.year - registered.year) * 12
        + as_of.month
        - registered.month
        - int(as_of.day < anniversary_day)
    )


@pytest.mark.parametrize(
    "registered,as_of,expected",
    [
        ("2023-10-22", "2026-10-22", 36),
        ("2023-10-23", "2026-10-22", 35),
        ("2024-02-29", "2025-02-28", 12),
        ("2024-02-29", "2025-02-27", 11),
        ("2026-01-31", "2026-02-28", 1),
        ("2026-10-01", "2026-10-22", 0),
    ],
)
def test_fixture_calendar_convention(registered, as_of, expected):
    assert full_months(date.fromisoformat(registered), date.fromisoformat(as_of)) == expected


def test_future_registration_cannot_be_an_age_zero_example():
    with pytest.raises(ValueError):
        full_months(date(2026, 10, 23), date(2026, 10, 22))


def test_shared_examples_preserve_dates_presence_units_and_expected_totals():
    fixtures = json.loads(FIXTURES_PATH.read_text(encoding="utf-8"))
    assert fixtures["schema_version"] == "scoring-personas-v1"
    as_of = date.fromisoformat(fixtures["as_of_date"])
    cases = fixtures["cases"]
    assert len(cases) == 10
    assert len({case["name"] for case in cases}) == len(cases)
    application_ids = set()
    for case in cases:
        request = ParseDict(case["request"], scoring_pb2.ScoreRequest())
        application_id = UUID(request.application_id)
        assert application_id not in application_ids
        application_ids.add(application_id)
        assert UUID(request.product_id).version == 4
        assert request.feature_schema_version == "borrower-features-v1"
        registered = date.fromisoformat(case["company"]["registered_at"])
        assert case["company"]["status"] == "ACTIVE"
        assert request.features.HasField("company_age_months")
        assert request.features.company_age_months == full_months(registered, as_of)
        for name in ("overdue_count_12m", "max_overdue_days"):
            assert request.features.HasField(name)
        for name in ("revenue_12m", "profit_12m", "total_debt"):
            if request.features.HasField(name):
                money = getattr(request.features, name)
                assert money.currency == "RUB"
                assert abs(money.amount_kopecks) <= 9007199254740991
                assert name == "profit_12m" or money.amount_kopecks >= 0
        expected = case["expected"]
        if expected["assessment_status"] == "ASSESSED":
            assert not expected["data_issues"]
            assert (
                500 + sum(item["points"] for item in expected["factor_contributions"])
                == (expected["score"])
            )
        else:
            assert "score" not in expected and "grade" not in expected
            assert not expected["factor_contributions"]

    by_name = {case["name"]: case for case in cases}
    missing = ParseDict(by_name["missing_debt"]["request"], scoring_pb2.ScoreRequest())
    known_zero = ParseDict(by_name["zero_debt"]["request"], scoring_pb2.ScoreRequest())
    assert not missing.features.HasField("total_debt")
    assert known_zero.features.HasField("total_debt")
    assert known_zero.features.total_debt.amount_kopecks == 0
    assert by_name["score_zero"]["expected"]["score"] == 0
