"""Monotone expert scorecard, not a default-probability estimator."""

import hashlib
import json
from dataclasses import dataclass
from decimal import ROUND_HALF_UP, Context, Decimal, InvalidOperation, localcontext
from importlib.resources import files
from pathlib import Path

from .types import Assessment, BorrowerSnapshot, FactorContribution, Reason
from .validation import validate_snapshot

_CONTEXT = Context(prec=256, rounding=ROUND_HALF_UP)
_FACTOR_NAMES = ("business_age", "profit_margin", "bureau_debt_load", "payment_discipline")
_FUNCTION_DIRECTIONS = {
    "business_age": "decreasing",
    "profit_margin": "decreasing",
    "bureau_debt_load": "increasing",
    "overdue_days": "increasing",
    "overdue_count": "increasing",
}
_REASON_CODES = {
    "business_age": ("SHORT_HISTORY", "STABLE_BUSINESS"),
    "profit_margin": ("LOW_PROFIT_MARGIN", "HEALTHY_PROFIT_MARGIN"),
    "bureau_debt_load": ("HIGH_BUREAU_DEBT", "MANAGEABLE_BUREAU_DEBT"),
    "payment_discipline": ("RECENT_OVERDUE", "GOOD_PAYMENT_DISCIPLINE"),
}
_DESCRIPTIONS = {
    "SHORT_HISTORY": "Небольшой возраст бизнеса повышает риск относительно опорного уровня модели.",
    "STABLE_BUSINESS": (
        "Продолжительность работы бизнеса снижает риск относительно опорного уровня модели."
    ),
    "LOW_PROFIT_MARGIN": "Рентабельность повышает риск относительно опорного уровня модели.",
    "HEALTHY_PROFIT_MARGIN": "Рентабельность снижает риск относительно опорного уровня модели.",
    "HIGH_BUREAU_DEBT": (
        "Отношение долга БКИ к выручке повышает риск относительно опорного уровня модели."
    ),
    "MANAGEABLE_BUREAU_DEBT": (
        "Отношение долга БКИ к выручке снижает риск относительно опорного уровня модели."
    ),
    "RECENT_OVERDUE": (
        "История просрочек за 12 месяцев повышает риск относительно опорного уровня модели."
    ),
    "GOOD_PAYMENT_DISCIPLINE": (
        "Просрочки имеются; их показатели снижают риск относительно опорного уровня модели."
    ),
    "NO_OVERDUE": "По переданным данным просрочки за 12 месяцев отсутствуют.",
}


@dataclass(frozen=True, slots=True)
class RiskFunction:
    name: str
    direction: str
    knots: tuple[tuple[Decimal, Decimal], ...]


@dataclass(frozen=True, slots=True)
class FactorConfig:
    name: str
    weight: int


@dataclass(frozen=True, slots=True)
class ModelConfig:
    model_version: str
    feature_schema_version: str
    baseline_score: int
    config_checksum: str
    factors: tuple[FactorConfig, ...]
    risk_functions: tuple[RiskFunction, ...]


def _unique_object(pairs: list[tuple[str, object]]) -> dict[str, object]:
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate configuration key: {key}")
        result[key] = value
    return result


def _object(value: object, keys: set[str], label: str) -> dict:
    if not isinstance(value, dict) or set(value) != keys:
        raise ValueError(f"invalid {label} fields")
    return value


def _decimal(value: object) -> Decimal:
    if type(value) not in (int, str, Decimal):
        raise ValueError("knot values must be finite decimal numbers")
    try:
        result = Decimal(value)
    except InvalidOperation as error:
        raise ValueError("invalid decimal in configuration") from error
    if not result.is_finite() or (result != 0 and abs(result.adjusted()) > 12):
        raise ValueError("knot values must be finite decimals of supported magnitude")
    if len(result.as_tuple().digits) > 30:
        raise ValueError("knot values have excessive precision")
    return result


def load_model(path: Path | None = None) -> ModelConfig:
    """Validate once and hash the exact bytes deployed with this model."""
    resource = files("smart_credit_scoring").joinpath("models/scorecard-v1.json")
    if path is not None:
        raw = path.read_bytes()
    elif resource.is_file():
        raw = resource.read_bytes()
    else:
        # Editable installs retain the source artifact; wheels include it as a resource.
        raw = (Path(__file__).resolve().parents[2] / "models/scorecard-v1.json").read_bytes()
    try:
        document = json.loads(raw, parse_float=Decimal, object_pairs_hook=_unique_object)
    except (json.JSONDecodeError, UnicodeDecodeError, RecursionError) as error:
        raise ValueError("invalid scorecard JSON") from error
    document = _object(
        document,
        {
            "model_version",
            "feature_schema_version",
            "baseline_score",
            "score_range",
            "provenance",
            "source",
            "license",
            "factors",
            "risk_functions",
        },
        "scorecard",
    )
    if document["model_version"] != "monotone-scorecard-v1":
        raise ValueError("unsupported model version")
    if document["feature_schema_version"] != "borrower-features-v1":
        raise ValueError("unsupported feature schema version")
    if type(document["baseline_score"]) is not int or document["baseline_score"] != 500:
        raise ValueError("baseline_score must be 500")
    score_range = document["score_range"]
    if (
        not isinstance(score_range, list)
        or any(type(value) is not int for value in score_range)
        or score_range != [0, 1000]
    ):
        raise ValueError("score_range must be [0, 1000]")
    for field in ("provenance", "source", "license"):
        if not isinstance(document[field], str) or not document[field].strip():
            raise ValueError(f"model {field} is required")
    if not isinstance(document["factors"], list):
        raise ValueError("factors must be a list")
    factors = []
    for value in document["factors"]:
        factor = _object(value, {"name", "weight"}, "factor")
        weight = factor["weight"]
        if type(weight) is not int or weight <= 0 or weight % 2:
            raise ValueError("factor weights must be positive even integers")
        factors.append(FactorConfig(factor["name"], weight))
    if tuple(factor.name for factor in factors) != _FACTOR_NAMES:
        raise ValueError("factors must have the four known names in canonical order")
    if sum(factor.weight for factor in factors) != 1000:
        raise ValueError("factor weights must sum to 1000")

    if not isinstance(document["risk_functions"], list):
        raise ValueError("risk_functions must be a list")
    risk_functions = []
    for value in document["risk_functions"]:
        function = _object(value, {"name", "direction", "knots"}, "risk function")
        name, direction = function["name"], function["direction"]
        if not isinstance(name, str) or _FUNCTION_DIRECTIONS.get(name) != direction:
            raise ValueError("unknown risk function or invalid monotonic direction")
        if not isinstance(function["knots"], list) or len(function["knots"]) < 2:
            raise ValueError("risk functions require at least two knots")
        knots = []
        for pair in function["knots"]:
            if not isinstance(pair, list) or len(pair) != 2:
                raise ValueError("each knot must be a pair")
            knots.append((_decimal(pair[0]), _decimal(pair[1])))
        if any(not 0 <= risk <= 1 for _, risk in knots):
            raise ValueError("risk values must be within [0, 1]")
        if any(left[0] >= right[0] for left, right in zip(knots, knots[1:], strict=False)):
            raise ValueError("knot positions must increase strictly")
        if direction == "increasing":
            monotone = all(
                left[1] <= right[1] for left, right in zip(knots, knots[1:], strict=False)
            )
            endpoints = (Decimal(0), Decimal(1))
        else:
            monotone = all(
                left[1] >= right[1] for left, right in zip(knots, knots[1:], strict=False)
            )
            endpoints = (Decimal(1), Decimal(0))
        if not monotone or (knots[0][1], knots[-1][1]) != endpoints:
            raise ValueError("risk must be monotonic and reach both endpoint risks")
        if name != "profit_margin" and knots[0][0] != 0:
            raise ValueError("nonnegative risk functions must start at zero")
        risk_functions.append(RiskFunction(name, direction, tuple(knots)))
    if tuple(function.name for function in risk_functions) != tuple(_FUNCTION_DIRECTIONS):
        raise ValueError("risk functions must have known names in canonical order")
    return ModelConfig(
        document["model_version"],
        document["feature_schema_version"],
        500,
        hashlib.sha256(raw).hexdigest(),
        tuple(factors),
        tuple(risk_functions),
    )


def piecewise_risk(value: Decimal, knots: tuple[tuple[Decimal, Decimal], ...]) -> Decimal:
    """Interpolate a validated monotonic function with endpoint saturation."""
    with localcontext(_CONTEXT):
        if value <= knots[0][0]:
            return knots[0][1]
        if value >= knots[-1][0]:
            return knots[-1][1]
        for (left_x, left_y), (right_x, right_y) in zip(knots, knots[1:], strict=False):
            if value <= right_x:
                return left_y + (value - left_x) * (right_y - left_y) / (right_x - left_x)
    raise ValueError("unreachable interpolation interval")


def grade_for(score: int) -> str:
    """Map the complete integer score range onto the bank's existing grades."""
    if type(score) is not int or not 0 <= score <= 1000:
        raise ValueError("score must be an integer in [0, 1000]")
    for minimum, grade in ((800, "A"), (650, "B"), (500, "C"), (350, "D")):
        if score >= minimum:
            return grade
    return "E"


def _ratio_points(numerator: int, denominator: int, function: RiskFunction, weight: int) -> int:
    """Avoid rounding an input ratio before resolving exact half-point boundaries."""
    dividend, divisor = Decimal(numerator), Decimal(denominator)
    knots = function.knots
    if dividend <= knots[0][0] * divisor:
        value = Decimal(weight) * (Decimal("0.5") - knots[0][1])
    elif dividend >= knots[-1][0] * divisor:
        value = Decimal(weight) * (Decimal("0.5") - knots[-1][1])
    else:
        for (left_x, left_y), (right_x, right_y) in zip(knots, knots[1:], strict=False):
            if dividend <= right_x * divisor:
                span = (right_x - left_x) * divisor
                exact_numerator = Decimal(weight) * (
                    (Decimal("0.5") - left_y) * span
                    - (dividend - left_x * divisor) * (right_y - left_y)
                )
                value = exact_numerator / span
                break
    return int(value.quantize(Decimal(1), rounding=ROUND_HALF_UP))


def _reason_code(factor: FactorContribution, snapshot: BorrowerSnapshot) -> str:
    negative, positive = _REASON_CODES[factor.factor_name]
    if factor.points < 0:
        return negative
    if factor.factor_name == "payment_discipline" and snapshot.overdue_count_12m == 0:
        return "NO_OVERDUE"
    return positive


def assess(snapshot: BorrowerSnapshot, config: ModelConfig) -> Assessment:
    """Evaluate borrower risk without clocks, identifiers, randomness or I/O."""
    issues = validate_snapshot(snapshot)
    metadata = {
        "baseline_score": config.baseline_score,
        "model_version": config.model_version,
        "feature_schema_version": config.feature_schema_version,
        "config_checksum": config.config_checksum,
    }
    if issues:
        status = "INSUFFICIENT_DATA" if issues[0].code == "MISSING_VALUE" else "UNSUPPORTED_INPUT"
        return Assessment(status, None, "UNSPECIFIED", (), issues, (), **metadata)

    with localcontext(_CONTEXT):
        values = {
            "business_age": Decimal(snapshot.company_age_months),
            "profit_margin": Decimal(snapshot.profit_12m) / Decimal(snapshot.revenue_12m),
            "bureau_debt_load": Decimal(snapshot.total_debt) / Decimal(snapshot.revenue_12m),
            "overdue_days": Decimal(snapshot.max_overdue_days),
            "overdue_count": Decimal(snapshot.overdue_count_12m),
        }
        risks = {
            function.name: piecewise_risk(values[function.name], function.knots)
            for function in config.risk_functions
        }
        risks["payment_discipline"] = max(risks["overdue_days"], risks["overdue_count"])
        factors = []
        for factor in config.factors:
            if factor.name in ("profit_margin", "bureau_debt_load"):
                numerator = (
                    snapshot.profit_12m if factor.name == "profit_margin" else snapshot.total_debt
                )
                function = next(
                    value for value in config.risk_functions if value.name == factor.name
                )
                points = _ratio_points(numerator, snapshot.revenue_12m, function, factor.weight)
            else:
                points = int(
                    (Decimal(factor.weight) * (Decimal("0.5") - risks[factor.name])).quantize(
                        Decimal(1), rounding=ROUND_HALF_UP
                    )
                )
            contribution = FactorContribution(factor.name, points, "")
            description = (
                _DESCRIPTIONS[_reason_code(contribution, snapshot)]
                if points
                else "Фактор находится на опорном уровне модели; вклад в score равен нулю."
            )
            factors.append(FactorContribution(factor.name, points, description))

    reasons = [
        Reason(
            _reason_code(factor, snapshot),
            factor.description,
            "INCREASES_RISK" if factor.points < 0 else "DECREASES_RISK",
            abs(factor.points),
        )
        for factor in factors
        if factor.points
    ]
    reasons.sort(key=lambda reason: (-reason.contribution, reason.code))
    score = config.baseline_score + sum(factor.points for factor in factors)
    return Assessment(
        "ASSESSED", score, grade_for(score), tuple(reasons[:3]), (), tuple(factors), **metadata
    )
