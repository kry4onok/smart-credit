import hashlib
import json
from dataclasses import FrozenInstanceError, replace
from decimal import ROUND_DOWN, Decimal, Inexact, localcontext
from fractions import Fraction
from pathlib import Path

import pytest
from hypothesis import given
from hypothesis import strategies as st
from smart_credit_scoring.model import assess, grade_for, load_model, piecewise_risk
from smart_credit_scoring.types import BorrowerSnapshot

CONFIG_PATH = Path(__file__).parents[1] / "models/scorecard-v1.json"
MODEL = load_model(CONFIG_PATH)
COMPLETE = BorrowerSnapshot(1_200_000_000, 180_000_000, 300_000_000, 36, 0, 0)


@pytest.mark.parametrize(
    "age,profit,debt,count,days,expected,grade",
    [
        (36, 180_000_000, 300_000_000, 0, 0, 938, "A"),
        (36, 60_000_000, 1_200_000_000, 0, 0, 700, "B"),
        (12, 60_000_000, 1_200_000_000, 1, 7, 510, "C"),
        (12, 30_000_000, 1_200_000_000, 1, 7, 472, "D"),
        (6, -120_000_000, 2_400_000_000, 2, 60, 55, "E"),
        (36, 180_000_000, 0, 0, 0, 975, "A"),
        (0, -120_000_000, 2_400_000_000, 6, 7, 0, "E"),
        (36, 300_000_000, 0, 0, 0, 1000, "A"),
    ],
)
def test_independent_reference_examples(age, profit, debt, count, days, expected, grade):
    result = assess(BorrowerSnapshot(1_200_000_000, profit, debt, age, count, days), MODEL)
    assert (result.status, result.score, result.grade) == ("ASSESSED", expected, grade)
    assert result.data_issues == ()
    assert tuple(f.factor_name for f in result.factor_contributions) == (
        "business_age",
        "profit_margin",
        "bureau_debt_load",
        "payment_discipline",
    )
    assert result.baseline_score + sum(f.points for f in result.factor_contributions) == expected


@pytest.mark.parametrize(
    "score,grade",
    [
        (0, "E"),
        (349, "E"),
        (350, "D"),
        (499, "D"),
        (500, "C"),
        (649, "C"),
        (650, "B"),
        (799, "B"),
        (800, "A"),
        (1000, "A"),
    ],
)
def test_grade_boundaries(score, grade):
    assert grade_for(score) == grade


@pytest.mark.parametrize("invalid", [-1, 1001, True, 800.0, "800", None])
def test_invalid_grade_inputs(invalid):
    with pytest.raises(ValueError):
        grade_for(invalid)


def test_every_configured_knot_and_saturation():
    for function in MODEL.risk_functions:
        for x, risk in function.knots:
            assert piecewise_risk(x, function.knots) == risk
        assert piecewise_risk(function.knots[0][0] - 100, function.knots) == function.knots[0][1]
        assert piecewise_risk(function.knots[-1][0] + 100, function.knots) == function.knots[-1][1]


def test_interpolation_and_neighboring_values():
    knots = ((Decimal(0), Decimal(0)), (Decimal(2), Decimal(1)))
    assert piecewise_risk(Decimal("0.999999"), knots) == Decimal("0.4999995")
    assert piecewise_risk(Decimal(1), knots) == Decimal("0.5")
    assert piecewise_risk(Decimal("1.000001"), knots) == Decimal("0.5000005")


@pytest.mark.parametrize("debt,expected", [(149, -49), (150, -50), (151, -51), (152, -51)])
def test_negative_exact_half_points_do_not_round_toward_zero(debt, expected):
    result = assess(replace(COMPLETE, revenue_12m=150, profit_12m=15, total_debt=debt), MODEL)
    assert result.factor_contributions[2].points == expected


def test_positive_half_points_round_away_from_zero():
    result = assess(COMPLETE, MODEL)
    assert result.factor_contributions[2].points == 88


def _fraction_points(numerator: int, denominator: int, knots: tuple, weight: int) -> int:
    """Independent exact rational oracle for production Decimal calculations."""
    ratio = Fraction(numerator, denominator)
    if ratio <= knots[0][0]:
        risk = knots[0][1]
    elif ratio >= knots[-1][0]:
        risk = knots[-1][1]
    else:
        for (left_x, left_y), (right_x, right_y) in zip(knots, knots[1:], strict=False):
            if ratio <= right_x:
                risk = left_y + (ratio - left_x) * (right_y - left_y) / (right_x - left_x)
                break
    points = weight * (Fraction(1, 2) - risk)
    magnitude = abs(points)
    quotient, remainder = divmod(magnitude.numerator, magnitude.denominator)
    rounded = quotient + (2 * remainder >= magnitude.denominator)
    return -rounded if points < 0 else rounded


@given(
    revenue=st.integers(1, 9_007_199_254_740_991),
    profit=st.integers(-9_007_199_254_740_991, 9_007_199_254_740_991),
    debt=st.integers(0, 9_007_199_254_740_991),
)
def test_financial_points_match_independent_exact_rational_oracle(revenue, profit, debt):
    snapshot = replace(COMPLETE, revenue_12m=revenue, profit_12m=profit, total_debt=debt)
    result = assess(snapshot, MODEL)
    margin_knots = tuple(
        (Fraction(x), Fraction(y))
        for x, y in (
            ("-0.10", "1"),
            ("0", "0.8"),
            ("0.05", "0.5"),
            ("0.15", "0.1"),
            ("0.25", "0"),
        )
    )
    debt_knots = tuple(
        (Fraction(x), Fraction(y))
        for x, y in (
            ("0", "0"),
            ("0.25", "0.15"),
            ("0.5", "0.35"),
            ("1", "0.7"),
            ("2", "1"),
        )
    )
    assert result.factor_contributions[1].points == _fraction_points(
        profit, revenue, margin_knots, 250
    )
    assert result.factor_contributions[2].points == _fraction_points(debt, revenue, debt_knots, 250)


def test_ambient_decimal_context_does_not_change_result():
    expected = assess(COMPLETE, MODEL)
    with localcontext() as context:
        context.prec = 2
        context.rounding = ROUND_DOWN
        context.traps[Inexact] = True
        assert assess(COMPLETE, MODEL) == expected
        assert load_model(CONFIG_PATH) == MODEL
        assert piecewise_risk(Decimal(9), MODEL.risk_functions[0].knots) == Decimal("0.675")


@given(
    age=st.integers(0, 1000),
    profit=st.integers(-1_000_000_000, 1_000_000_000),
    debt=st.integers(0, 10_000_000_000),
    days=st.integers(1, 1000),
    count=st.integers(1, 100),
)
def test_factor_sum_bounds_and_reasons(age, profit, debt, days, count):
    result = assess(BorrowerSnapshot(1_200_000_000, profit, debt, age, count, days), MODEL)
    assert 0 <= result.score <= 1000
    assert result.score == 500 + sum(factor.points for factor in result.factor_contributions)
    assert len(result.factor_contributions) == 4
    assert len(result.reasons) <= 3
    assert list(result.reasons) == sorted(
        result.reasons, key=lambda reason: (-reason.contribution, reason.code)
    )
    assert all(reason.contribution > 0 for reason in result.reasons)
    signed = {factor.description: factor.points for factor in result.factor_contributions}
    for reason in result.reasons:
        assert reason.contribution == abs(signed[reason.description])
        assert reason.direction == (
            "INCREASES_RISK" if signed[reason.description] < 0 else "DECREASES_RISK"
        )


@given(
    age=st.integers(0, 200),
    profit=st.integers(-1_000_000_000, 1_000_000_000),
    debt=st.integers(0, 10_000_000_000),
    days=st.integers(1, 100),
    count=st.integers(1, 10),
    delta=st.integers(0, 100_000),
)
def test_monotonicity(age, profit, debt, days, count, delta):
    snapshot = BorrowerSnapshot(1_200_000_000, profit, debt, age, count, days)
    base = assess(snapshot, MODEL).score
    assert assess(replace(snapshot, company_age_months=age + delta), MODEL).score >= base
    assert assess(replace(snapshot, profit_12m=profit + delta), MODEL).score >= base
    assert assess(replace(snapshot, total_debt=debt + delta), MODEL).score <= base
    assert assess(replace(snapshot, max_overdue_days=days + delta), MODEL).score <= base
    assert assess(replace(snapshot, overdue_count_12m=count + delta), MODEL).score <= base


@given(multiplier=st.integers(1, 1_000_000))
def test_monetary_scale_invariance(multiplier):
    scaled = replace(
        COMPLETE,
        revenue_12m=COMPLETE.revenue_12m * multiplier,
        profit_12m=COMPLETE.profit_12m * multiplier,
        total_debt=COMPLETE.total_debt * multiplier,
    )
    assert assess(scaled, MODEL) == assess(COMPLETE, MODEL)


def test_payment_discipline_uses_maximum_instead_of_double_penalty():
    one_source = assess(replace(COMPLETE, overdue_count_12m=1, max_overdue_days=90), MODEL)
    both_sources = assess(replace(COMPLETE, overdue_count_12m=6, max_overdue_days=90), MODEL)
    assert one_source.factor_contributions[3].points == -200
    assert one_source == both_sources


def test_no_overdue_reason_requires_explicit_zero_history():
    clear = assess(COMPLETE, MODEL)
    assert clear.reasons[0].code == "NO_OVERDUE"
    small_overdue = assess(replace(COMPLETE, overdue_count_12m=1, max_overdue_days=1), MODEL)
    assert "GOOD_PAYMENT_DISCIPLINE" in {reason.code for reason in small_overdue.reasons}
    assert "NO_OVERDUE" not in {reason.code for reason in small_overdue.reasons}


def test_reason_ties_sort_by_unprefixed_code():
    snapshot = replace(
        COMPLETE, company_age_months=36, profit_12m=60_000_000, total_debt=1_200_000_000
    )
    result = assess(snapshot, MODEL)
    assert [(reason.code, reason.contribution) for reason in result.reasons] == [
        ("NO_OVERDUE", 200),
        ("HIGH_BUREAU_DEBT", 50),
        ("STABLE_BUSINESS", 50),
    ]
    assert result.factor_contributions[1].points == 0


def test_inputs_outputs_and_nested_model_are_immutable():
    before = replace(COMPLETE)
    result = assess(COMPLETE, MODEL)
    assert COMPLETE == before
    for obj, name in [
        (COMPLETE, "total_debt"),
        (MODEL, "baseline_score"),
        (MODEL.factors[0], "weight"),
        (result, "score"),
    ]:
        with pytest.raises(FrozenInstanceError):
            setattr(obj, name, 1)


def test_checksum_hashes_actual_file_bytes(tmp_path):
    raw = CONFIG_PATH.read_bytes()
    changed = tmp_path / "config.json"
    changed.write_bytes(raw + b"\n")
    model = load_model(changed)
    assert MODEL.config_checksum == hashlib.sha256(raw).hexdigest()
    assert model.config_checksum == hashlib.sha256(raw + b"\n").hexdigest()
    assert model.config_checksum != MODEL.config_checksum
    assert assess(COMPLETE, model).score == assess(COMPLETE, MODEL).score


@pytest.mark.parametrize(
    "change",
    [
        lambda doc: doc.update(model_version="unknown"),
        lambda doc: doc.update(feature_schema_version="unknown"),
        lambda doc: doc.update(baseline_score=True),
        lambda doc: doc.update(score_range=[0, True]),
        lambda doc: doc.update(provenance=""),
        lambda doc: doc.update(unexpected=True),
        lambda doc: doc["factors"][0].update(weight=-100),
        lambda doc: doc["factors"][0].update(weight=101),
        lambda doc: doc["factors"][0].update(weight=102),
        lambda doc: doc["factors"][0].update(name="other"),
        lambda doc: doc.update(factors=doc["factors"][1:]),
        lambda doc: doc["risk_functions"][0].update(direction="increasing"),
        lambda doc: doc["risk_functions"][0].update(knots=[]),
        lambda doc: doc["risk_functions"][0].update(knots=[["0", "1"], ["0", "0"]]),
        lambda doc: doc["risk_functions"][0].update(
            knots=[["0", "1"], ["1", "0.2"], ["2", "0.3"], ["3", "0"]]
        ),
        lambda doc: doc["risk_functions"][0].update(knots=[["0", "1"], ["1", "-0.1"]]),
        lambda doc: doc["risk_functions"][0].update(knots=[["0", "1"], ["1", "0.2"]]),
        lambda doc: doc["risk_functions"][0].update(knots=[["0", "NaN"], ["1", "0"]]),
        lambda doc: doc["risk_functions"][0].update(knots=[["0", "Infinity"], ["1", "0"]]),
        lambda doc: doc["risk_functions"][0].update(knots=[["0", True], ["1", "0"]]),
        lambda doc: doc["risk_functions"][0].update(knots=[["1", "1"], ["2", "0"]]),
        lambda doc: doc.update(risk_functions=doc["risk_functions"][:-1]),
    ],
)
def test_malformed_configuration_fails_startup(tmp_path, change):
    document = json.loads(CONFIG_PATH.read_bytes())
    change(document)
    target = tmp_path / "bad.json"
    target.write_text(json.dumps(document), encoding="utf-8")
    with pytest.raises(ValueError):
        load_model(target)


def test_duplicate_json_keys_are_rejected(tmp_path):
    target = tmp_path / "duplicate.json"
    target.write_text('{"model_version":"first","model_version":"second"}')
    with pytest.raises(ValueError, match="duplicate"):
        load_model(target)


@pytest.mark.parametrize("contents", [b"not json", b"\xff"])
def test_invalid_json_encoding_or_syntax_fails_startup(tmp_path, contents):
    target = tmp_path / "invalid.json"
    target.write_bytes(contents)
    with pytest.raises(ValueError, match="JSON"):
        load_model(target)
