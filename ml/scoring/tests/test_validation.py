from dataclasses import replace

import pytest
from smart_credit_scoring.model import assess, load_model
from smart_credit_scoring.types import BorrowerSnapshot
from smart_credit_scoring.validation import MAX_MONEY_KOPECKS, InvalidInput, validate_snapshot

COMPLETE = BorrowerSnapshot(1_200_000_000, 180_000_000, 300_000_000, 36, 0, 0)


def test_known_zero_is_not_missing():
    assert (
        validate_snapshot(replace(COMPLETE, total_debt=0, profit_12m=0, company_age_months=0)) == ()
    )


@pytest.mark.parametrize("name", BorrowerSnapshot.__dataclass_fields__)
def test_each_missing_field_is_explicit(name):
    issues = validate_snapshot(replace(COMPLETE, **{name: None}))
    assert [(issue.code, issue.field_path) for issue in issues] == [
        ("MISSING_VALUE", f"features.{name}")
    ]


def test_all_missing_fields_are_sorted():
    issues = validate_snapshot(BorrowerSnapshot())
    assert len(issues) == 6
    assert [issue.field_path for issue in issues] == sorted(issue.field_path for issue in issues)


@pytest.mark.parametrize("name", BorrowerSnapshot.__dataclass_fields__)
@pytest.mark.parametrize("bad_value", [True, False, 1.0, "1", [], {}])
def test_invalid_types_are_not_coerced(name, bad_value):
    with pytest.raises(InvalidInput, match=name):
        validate_snapshot(replace(COMPLETE, **{name: bad_value}))


@pytest.mark.parametrize(
    "name",
    ["revenue_12m", "total_debt", "company_age_months", "overdue_count_12m", "max_overdue_days"],
)
def test_negative_unsigned_inputs_are_invalid(name):
    with pytest.raises(InvalidInput, match=name):
        validate_snapshot(replace(COMPLETE, **{name: -1}))


@pytest.mark.parametrize("name", ["revenue_12m", "profit_12m", "total_debt"])
def test_money_bounds(name):
    assert validate_snapshot(replace(COMPLETE, **{name: MAX_MONEY_KOPECKS})) == ()
    with pytest.raises(InvalidInput, match=name):
        validate_snapshot(replace(COMPLETE, **{name: MAX_MONEY_KOPECKS + 1}))
    if name == "profit_12m":
        assert validate_snapshot(replace(COMPLETE, profit_12m=-MAX_MONEY_KOPECKS)) == ()
        with pytest.raises(InvalidInput, match=name):
            validate_snapshot(replace(COMPLETE, profit_12m=-MAX_MONEY_KOPECKS - 1))


@pytest.mark.parametrize("name", ["company_age_months", "overdue_count_12m", "max_overdue_days"])
def test_int32_scalar_bounds(name):
    with pytest.raises(InvalidInput, match=name):
        validate_snapshot(replace(COMPLETE, **{name: 2_147_483_648}))


@pytest.mark.parametrize("count,days", [(0, 1), (1, 0)])
def test_inconsistent_overdue(count, days):
    issues = validate_snapshot(replace(COMPLETE, overdue_count_12m=count, max_overdue_days=days))
    assert [(issue.code, issue.field_path) for issue in issues] == [
        ("INCONSISTENT_OVERDUE", "features.max_overdue_days")
    ]


def test_ongoing_overdue_can_exceed_a_year():
    assert validate_snapshot(replace(COMPLETE, overdue_count_12m=1, max_overdue_days=800)) == ()


def test_structural_error_precedes_missing_and_unsupported():
    with pytest.raises(InvalidInput, match="profit_12m"):
        validate_snapshot(replace(COMPLETE, revenue_12m=0, profit_12m=True, total_debt=None))


def test_missing_precedes_unsupported():
    issues = validate_snapshot(
        replace(COMPLETE, revenue_12m=0, total_debt=None, max_overdue_days=7)
    )
    assert [(issue.code, issue.field_path) for issue in issues] == [
        ("MISSING_VALUE", "features.total_debt")
    ]


def test_multiple_unsupported_issues_are_sorted():
    issues = validate_snapshot(replace(COMPLETE, revenue_12m=0, max_overdue_days=7))
    assert [(issue.code, issue.field_path) for issue in issues] == [
        ("INCONSISTENT_OVERDUE", "features.max_overdue_days"),
        ("ZERO_REVENUE", "features.revenue_12m"),
    ]


@pytest.mark.parametrize(
    "snapshot,status",
    [
        (replace(COMPLETE, total_debt=None), "INSUFFICIENT_DATA"),
        (replace(COMPLETE, revenue_12m=0), "UNSUPPORTED_INPUT"),
        (replace(COMPLETE, max_overdue_days=1), "UNSUPPORTED_INPUT"),
    ],
)
def test_abstention_preserves_metadata_without_synthetic_score(snapshot, status):
    model = load_model()
    result = assess(snapshot, model)
    assert result.status == status
    assert result.score is None
    assert result.grade == "UNSPECIFIED"
    assert result.reasons == result.factor_contributions == ()
    assert result.data_issues
    assert result.model_version == model.model_version
    assert result.feature_schema_version == model.feature_schema_version
    assert result.config_checksum == model.config_checksum
    assert result.baseline_score == 500


def test_snapshot_type_is_checked():
    with pytest.raises(InvalidInput, match="BorrowerSnapshot"):
        validate_snapshot({})
