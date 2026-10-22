"""Input validation with structural errors preceding domain abstention."""

from dataclasses import fields

from .types import BorrowerSnapshot, DataIssue

MAX_MONEY_KOPECKS = 9_007_199_254_740_991
MAX_INT32 = 2_147_483_647
MONEY_FIELDS = frozenset({"revenue_12m", "profit_12m", "total_debt"})


class InvalidInput(ValueError):
    """A malformed value must be reported as an invalid transport argument."""


def validate_snapshot(snapshot: BorrowerSnapshot) -> tuple[DataIssue, ...]:
    """Return missing or unsupported issues; reject malformed present values first."""
    if not isinstance(snapshot, BorrowerSnapshot):
        raise InvalidInput("features must be a BorrowerSnapshot")

    missing = []
    for field in fields(snapshot):
        value = getattr(snapshot, field.name)
        if value is None:
            missing.append(DataIssue("MISSING_VALUE", f"features.{field.name}"))
            continue
        if type(value) is not int:
            raise InvalidInput(f"features.{field.name} must be an integer")
        if field.name != "profit_12m" and value < 0:
            raise InvalidInput(f"features.{field.name} must be nonnegative")
        limit = MAX_MONEY_KOPECKS if field.name in MONEY_FIELDS else MAX_INT32
        if abs(value) > limit:
            raise InvalidInput(f"features.{field.name} exceeds the supported integer range")

    if missing:
        return tuple(sorted(missing, key=lambda issue: (issue.field_path, issue.code)))

    issues = []
    if snapshot.revenue_12m == 0:
        issues.append(DataIssue("ZERO_REVENUE", "features.revenue_12m"))
    if (snapshot.overdue_count_12m == 0) != (snapshot.max_overdue_days == 0):
        issues.append(DataIssue("INCONSISTENT_OVERDUE", "features.max_overdue_days"))
    return tuple(sorted(issues, key=lambda issue: (issue.field_path, issue.code)))
