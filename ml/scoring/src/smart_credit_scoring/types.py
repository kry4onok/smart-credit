"""Immutable domain values, independent of protobuf and service lifecycle."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class BorrowerSnapshot:
    """Amounts are integer kopecks; None preserves unavailable information."""

    revenue_12m: int | None = None
    profit_12m: int | None = None
    total_debt: int | None = None
    company_age_months: int | None = None
    overdue_count_12m: int | None = None
    max_overdue_days: int | None = None


@dataclass(frozen=True, slots=True)
class DataIssue:
    code: str
    field_path: str


@dataclass(frozen=True, slots=True)
class FactorContribution:
    factor_name: str
    points: int
    description: str


@dataclass(frozen=True, slots=True)
class Reason:
    code: str
    description: str
    direction: str
    contribution: int


@dataclass(frozen=True, slots=True)
class Assessment:
    status: str
    score: int | None
    grade: str
    reasons: tuple[Reason, ...]
    data_issues: tuple[DataIssue, ...]
    factor_contributions: tuple[FactorContribution, ...]
    baseline_score: int
    model_version: str
    feature_schema_version: str
    config_checksum: str
