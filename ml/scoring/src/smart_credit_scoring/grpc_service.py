"""Bounded gRPC adapter for the pure scoring model."""

import json
import logging
import os
import re
import signal
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from uuid import UUID

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from proto.scoring import scoring_pb2, scoring_pb2_grpc

from .model import ModelConfig, assess, load_model
from .types import Assessment, BorrowerSnapshot
from .validation import InvalidInput

LOGGER = logging.getLogger(__name__)
SERVICE_NAME = "smartcredit.scoring.ScoringService"
MAX_MESSAGE_BYTES = 64 * 1024
_SAFE_IDENTIFIER = re.compile(r"[A-Za-z0-9._:-]{1,128}\Z")
_MONEY_FIELDS = ("revenue_12m", "profit_12m", "total_debt")
_SCALAR_FIELDS = ("company_age_months", "overdue_count_12m", "max_overdue_days")


def _valid_uuid(value: str) -> bool:
    try:
        return str(UUID(value)) == value.lower()
    except (ValueError, AttributeError):
        return False


def _snapshot(request: scoring_pb2.ScoreRequest) -> BorrowerSnapshot:
    values = {}
    for name in _MONEY_FIELDS:
        if not request.features.HasField(name):
            values[name] = None
            continue
        money = getattr(request.features, name)
        if money.currency != "RUB":
            raise InvalidInput(f"features.{name}.currency must be RUB")
        values[name] = money.amount_kopecks
    for name in _SCALAR_FIELDS:
        values[name] = getattr(request.features, name) if request.features.HasField(name) else None
    return BorrowerSnapshot(**values)


def _response(result: Assessment) -> scoring_pb2.ScoreResponse:
    response = scoring_pb2.ScoreResponse(
        assessment_status=scoring_pb2.AssessmentStatus.Value(f"ASSESSMENT_STATUS_{result.status}"),
        grade=scoring_pb2.Grade.Value(f"GRADE_{result.grade}"),
        model_version=result.model_version,
        feature_schema_version=result.feature_schema_version,
        baseline_score=result.baseline_score,
        config_checksum=result.config_checksum,
        data_issues=[
            scoring_pb2.DataIssue(code=issue.code, field_path=issue.field_path)
            for issue in result.data_issues
        ],
        factor_contributions=[
            scoring_pb2.FactorContribution(
                factor_name=factor.factor_name, points=factor.points, description=factor.description
            )
            for factor in result.factor_contributions
        ],
        reasons=[
            scoring_pb2.Reason(
                code=scoring_pb2.ReasonCode.Value(f"REASON_CODE_{reason.code}"),
                description=reason.description,
                direction=scoring_pb2.Direction.Value(f"DIRECTION_{reason.direction}"),
                contribution=reason.contribution,
            )
            for reason in result.reasons
        ],
    )
    if result.score is not None:
        response.score = result.score
    return response


def _identifiers(request: scoring_pb2.ScoreRequest, context: grpc.ServicerContext) -> dict:
    identifiers = {}
    if _valid_uuid(request.application_id):
        identifiers["application_id"] = request.application_id
    allowed = {
        "attempt-id": "attempt_id",
        "x-attempt-id": "attempt_id",
        "trace-id": "trace_id",
        "x-trace-id": "trace_id",
    }
    for key, value in context.invocation_metadata():
        if key in allowed and isinstance(value, str) and _SAFE_IDENTIFIER.fullmatch(value):
            identifiers.setdefault(allowed[key], value)
    return identifiers


class ScoringService(scoring_pb2_grpc.ScoringServiceServicer):
    """Validate the envelope, preserve field presence, and map domain outcomes."""

    def __init__(self, config: ModelConfig):
        self.config = config

    def Score(self, request, context):  # noqa: N802 - protobuf service method name.
        started = time.perf_counter()
        outcome = "INTERNAL"
        try:
            for field in ("application_id", "product_id"):
                if not _valid_uuid(getattr(request, field)):
                    outcome = "INVALID_ARGUMENT"
                    context.abort(grpc.StatusCode.INVALID_ARGUMENT, f"{field} must be a UUID")
            if not request.feature_schema_version:
                outcome = "INVALID_ARGUMENT"
                context.abort(
                    grpc.StatusCode.INVALID_ARGUMENT, "feature_schema_version is required"
                )
            if request.feature_schema_version != self.config.feature_schema_version:
                outcome = "FAILED_PRECONDITION"
                context.abort(
                    grpc.StatusCode.FAILED_PRECONDITION, "unsupported feature_schema_version"
                )
            result = assess(_snapshot(request), self.config)
            response = _response(result)
            outcome = result.status
            return response
        except InvalidInput as error:
            outcome = "INVALID_ARGUMENT"
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(error))
        except Exception:
            # context.abort raises internally; retain the deliberate public error.
            if outcome in ("INVALID_ARGUMENT", "FAILED_PRECONDITION"):
                raise
            context.abort(grpc.StatusCode.INTERNAL, "scoring failed")
        finally:
            LOGGER.info(
                json.dumps(
                    {
                        "event": "scoring_request",
                        **_identifiers(request, context),
                        "model_version": self.config.model_version,
                        "feature_schema_version": self.config.feature_schema_version,
                        "duration_ms": round((time.perf_counter() - started) * 1000, 3),
                        "outcome": outcome,
                    },
                    ensure_ascii=True,
                    separators=(",", ":"),
                )
            )


def create_server(
    config: ModelConfig, address: str = "127.0.0.1:0"
) -> tuple[grpc.Server, health.HealthServicer, int]:
    """Build an unstarted server; the caller owns start and graceful shutdown."""
    server = grpc.server(
        ThreadPoolExecutor(max_workers=8, thread_name_prefix="scoring"),
        maximum_concurrent_rpcs=64,
        options=(
            ("grpc.max_receive_message_length", MAX_MESSAGE_BYTES),
            ("grpc.max_send_message_length", MAX_MESSAGE_BYTES),
        ),
    )
    scoring_pb2_grpc.add_ScoringServiceServicer_to_server(ScoringService(config), server)
    health_service = health.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(health_service, server)
    port = server.add_insecure_port(address)
    if port == 0:
        raise RuntimeError("could not bind scoring address")
    for name in ("", SERVICE_NAME):
        health_service.set(name, health_pb2.HealthCheckResponse.SERVING)
    return server, health_service, port


def serve() -> None:
    """Load the model before opening a port and stop cleanly on SIGTERM/SIGINT."""
    logging.basicConfig(level=logging.INFO, format="%(message)s")
    try:
        configured_path = os.environ.get("SCORING_MODEL_PATH")
        config = load_model(Path(configured_path) if configured_path else None)
        server, health_service, _ = create_server(
            config, os.environ.get("SCORING_ADDR", "127.0.0.1:50051")
        )
    except Exception:
        LOGGER.error('{"event":"scoring_startup","outcome":"INVALID_CONFIGURATION"}')
        raise SystemExit(1) from None

    stopping = threading.Event()

    def stop_requested(_signum, _frame):
        stopping.set()

    for signum in (signal.SIGTERM, signal.SIGINT):
        signal.signal(signum, stop_requested)
    server.start()
    LOGGER.info(
        json.dumps(
            {
                "event": "scoring_startup",
                "outcome": "SERVING",
                "model_version": config.model_version,
                "feature_schema_version": config.feature_schema_version,
                "config_checksum": config.config_checksum,
            },
            separators=(",", ":"),
        )
    )
    try:
        stopping.wait()
    finally:
        health_service.enter_graceful_shutdown()
        server.stop(grace=5).wait()
        LOGGER.info('{"event":"scoring_shutdown","outcome":"STOPPED"}')


if __name__ == "__main__":
    serve()
