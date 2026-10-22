"""Exercise the generated client against a real local gRPC server."""

import json
import logging
import os
import signal
import socket
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import grpc
import pytest
from grpc_health.v1 import health_pb2, health_pb2_grpc
from smart_credit_scoring import grpc_service
from smart_credit_scoring.model import load_model

from proto.common import common_pb2
from proto.scoring import scoring_pb2, scoring_pb2_grpc


@pytest.fixture
def score_request():
    return scoring_pb2.ScoreRequest(
        application_id="c1000000-0000-4000-8000-000000000001",
        product_id="11111111-1111-4111-8111-111111111111",
        feature_schema_version="borrower-features-v1",
        features=scoring_pb2.Features(
            revenue_12m=common_pb2.Money(amount_kopecks=1_200_000_000, currency="RUB"),
            profit_12m=common_pb2.Money(amount_kopecks=180_000_000, currency="RUB"),
            total_debt=common_pb2.Money(amount_kopecks=300_000_000, currency="RUB"),
            company_age_months=36,
            overdue_count_12m=0,
            max_overdue_days=0,
        ),
    )


@pytest.fixture
def running_server():
    server, health, port = grpc_service.create_server(load_model())
    server.start()
    with grpc.insecure_channel(f"127.0.0.1:{port}") as channel:
        grpc.channel_ready_future(channel).result(timeout=3)
        yield scoring_pb2_grpc.ScoringServiceStub(channel), channel, health
    health.enter_graceful_shutdown()
    server.stop(grace=0).wait(timeout=3)


def test_assessed_response_and_factor_accounting(running_server, score_request):
    stub, _, _ = running_server
    response = stub.Score(score_request, timeout=2)
    assert response.assessment_status == scoring_pb2.ASSESSMENT_STATUS_ASSESSED
    assert response.HasField("score") and response.score == 938
    assert response.grade == scoring_pb2.GRADE_A
    assert response.model_version == "monotone-scorecard-v1"
    assert response.feature_schema_version == "borrower-features-v1"
    assert response.config_checksum == load_model().config_checksum
    assert not response.data_issues
    assert len(response.factor_contributions) == 4
    assert (
        response.baseline_score + sum(item.points for item in response.factor_contributions)
        == response.score
    )
    assert len(response.reasons) == 3
    assert all(item.contribution > 0 and item.description for item in response.reasons)


def test_assessed_zero_is_present_after_wire_roundtrip(running_server, score_request):
    stub, _, _ = running_server
    score_request.features.profit_12m.amount_kopecks = -120_000_000
    score_request.features.total_debt.amount_kopecks = 2_400_000_000
    score_request.features.company_age_months = 0
    score_request.features.overdue_count_12m = 6
    score_request.features.max_overdue_days = 7
    response = stub.Score(score_request, timeout=2)
    assert response.HasField("score") and response.score == 0
    assert response.grade == scoring_pb2.GRADE_E
    assert response.assessment_status == scoring_pb2.ASSESSMENT_STATUS_ASSESSED


@pytest.mark.parametrize(
    "field",
    [
        "revenue_12m",
        "profit_12m",
        "total_debt",
        "company_age_months",
        "overdue_count_12m",
        "max_overdue_days",
    ],
)
def test_missing_fields_return_normal_abstention(running_server, score_request, field):
    stub, _, _ = running_server
    score_request.features.ClearField(field)
    response = stub.Score(score_request, timeout=2)
    assert response.assessment_status == scoring_pb2.ASSESSMENT_STATUS_INSUFFICIENT_DATA
    assert not response.HasField("score")
    assert response.grade == scoring_pb2.GRADE_UNSPECIFIED
    assert not response.reasons and not response.factor_contributions
    assert [(issue.field_path, issue.code) for issue in response.data_issues] == [
        (f"features.{field}", "MISSING_VALUE")
    ]
    assert response.baseline_score == 500 and len(response.config_checksum) == 64


def test_absent_features_reports_all_missing(running_server, score_request):
    stub, _, _ = running_server
    score_request.ClearField("features")
    response = stub.Score(score_request, timeout=2)
    assert response.assessment_status == scoring_pb2.ASSESSMENT_STATUS_INSUFFICIENT_DATA
    assert len(response.data_issues) == 6
    assert [item.field_path for item in response.data_issues] == sorted(
        item.field_path for item in response.data_issues
    )


@pytest.mark.parametrize("count,days", [(0, 7), (1, 0)])
def test_conflicting_overdue_is_unsupported(running_server, score_request, count, days):
    stub, _, _ = running_server
    score_request.features.overdue_count_12m = count
    score_request.features.max_overdue_days = days
    response = stub.Score(score_request, timeout=2)
    assert response.assessment_status == scoring_pb2.ASSESSMENT_STATUS_UNSUPPORTED_INPUT
    assert not response.HasField("score")
    assert response.data_issues[0].code == "INCONSISTENT_OVERDUE"


def test_missing_precedes_unsupported(running_server, score_request):
    stub, _, _ = running_server
    score_request.features.revenue_12m.amount_kopecks = 0
    score_request.features.ClearField("total_debt")
    response = stub.Score(score_request, timeout=2)
    assert response.assessment_status == scoring_pb2.ASSESSMENT_STATUS_INSUFFICIENT_DATA
    assert [issue.code for issue in response.data_issues] == ["MISSING_VALUE"]


def test_known_zero_is_not_missing(running_server, score_request):
    stub, _, _ = running_server
    score_request.features.total_debt.amount_kopecks = 0
    response = stub.Score(score_request, timeout=2)
    assert response.score == 975
    score_request.features.revenue_12m.amount_kopecks = 0
    response = stub.Score(score_request, timeout=2)
    assert response.assessment_status == scoring_pb2.ASSESSMENT_STATUS_UNSUPPORTED_INPUT
    assert not response.HasField("score")
    assert [issue.code for issue in response.data_issues] == ["ZERO_REVENUE"]


@pytest.mark.parametrize(
    "field,value,code",
    [
        ("application_id", "", grpc.StatusCode.INVALID_ARGUMENT),
        ("application_id", "not-a-uuid", grpc.StatusCode.INVALID_ARGUMENT),
        ("product_id", "not-a-uuid", grpc.StatusCode.INVALID_ARGUMENT),
        ("feature_schema_version", "", grpc.StatusCode.INVALID_ARGUMENT),
        ("feature_schema_version", "future-v2", grpc.StatusCode.FAILED_PRECONDITION),
    ],
)
def test_envelope_errors(running_server, score_request, field, value, code):
    stub, _, _ = running_server
    setattr(score_request, field, value)
    with pytest.raises(grpc.RpcError) as error:
        stub.Score(score_request, timeout=2)
    assert error.value.code() == code


@pytest.mark.parametrize("field", ["revenue_12m", "profit_12m", "total_debt"])
def test_currency_is_strict_even_when_other_fields_are_missing(
    running_server, score_request, field
):
    stub, _, _ = running_server
    getattr(score_request.features, field).currency = "USD"
    score_request.features.ClearField("company_age_months")
    with pytest.raises(grpc.RpcError) as error:
        stub.Score(score_request, timeout=2)
    assert error.value.code() == grpc.StatusCode.INVALID_ARGUMENT
    assert error.value.details() == f"features.{field}.currency must be RUB"


@pytest.mark.parametrize(
    "field,value",
    [
        ("revenue_12m", -1),
        ("total_debt", -1),
        ("profit_12m", 9_007_199_254_740_992),
        ("company_age_months", -1),
        ("overdue_count_12m", -1),
        ("max_overdue_days", -1),
    ],
)
def test_structural_values_precede_missing(running_server, score_request, field, value):
    stub, _, _ = running_server
    if field in ("revenue_12m", "profit_12m", "total_debt"):
        getattr(score_request.features, field).amount_kopecks = value
    else:
        setattr(score_request.features, field, value)
    score_request.features.ClearField(
        "profit_12m" if field != "profit_12m" else "company_age_months"
    )
    with pytest.raises(grpc.RpcError) as error:
        stub.Score(score_request, timeout=2)
    assert error.value.code() == grpc.StatusCode.INVALID_ARGUMENT


def test_legacy_fields_are_ignored(running_server, score_request):
    stub, _, _ = running_server
    before = stub.Score(score_request, timeout=2)
    score_request.features.obligations.CopyFrom(
        common_pb2.Money(amount_kopecks=-100, currency="USD")
    )
    score_request.features.requested_amount.CopyFrom(
        common_pb2.Money(amount_kopecks=-1, currency="EUR")
    )
    score_request.features.okved_group = "unknown"
    score_request.features.requested_term_months = -1
    assert stub.Score(score_request, timeout=2) == before


def test_health_serving_and_graceful_shutdown(running_server):
    _, channel, health = running_server
    stub = health_pb2_grpc.HealthStub(channel)
    for service in ("", grpc_service.SERVICE_NAME):
        assert stub.Check(health_pb2.HealthCheckRequest(service=service), timeout=2).status == (
            health_pb2.HealthCheckResponse.SERVING
        )
    with pytest.raises(grpc.RpcError) as error:
        stub.Check(health_pb2.HealthCheckRequest(service="unknown"), timeout=2)
    assert error.value.code() == grpc.StatusCode.NOT_FOUND
    health.enter_graceful_shutdown()
    assert stub.Check(health_pb2.HealthCheckRequest(), timeout=2).status == (
        health_pb2.HealthCheckResponse.NOT_SERVING
    )


def test_real_deadline(running_server, score_request, monkeypatch):
    stub, _, _ = running_server
    original = grpc_service.assess

    def slow_assessment(snapshot, config):
        time.sleep(0.1)
        return original(snapshot, config)

    monkeypatch.setattr(grpc_service, "assess", slow_assessment)
    with pytest.raises(grpc.RpcError) as error:
        stub.Score(score_request, timeout=0.01)
    assert error.value.code() == grpc.StatusCode.DEADLINE_EXCEEDED


def test_concurrent_requests_are_deterministic(running_server, score_request):
    stub, _, _ = running_server
    with ThreadPoolExecutor(max_workers=10) as pool:
        responses = list(pool.map(lambda _: stub.Score(score_request, timeout=2), range(30)))
    assert all(response == responses[0] for response in responses)


def test_message_size_is_bounded(running_server, score_request):
    stub, _, _ = running_server
    score_request.features.okved_group = "x" * (grpc_service.MAX_MESSAGE_BYTES + 1)
    with pytest.raises(grpc.RpcError) as error:
        stub.Score(score_request, timeout=2)
    assert error.value.code() == grpc.StatusCode.RESOURCE_EXHAUSTED


def test_unexpected_failure_is_sanitized(running_server, score_request, monkeypatch, caplog):
    stub, _, _ = running_server

    def failure(_snapshot, _config):
        raise RuntimeError("secret financial payload 180000000")

    monkeypatch.setattr(grpc_service, "assess", failure)
    with caplog.at_level(logging.INFO), pytest.raises(grpc.RpcError) as error:
        stub.Score(score_request, timeout=2)
    assert error.value.code() == grpc.StatusCode.INTERNAL
    assert error.value.details() == "scoring failed"
    assert "secret" not in caplog.text and "180000000" not in caplog.text
    assert json.loads(caplog.records[-1].message)["outcome"] == "INTERNAL"


def test_logs_only_safe_identifiers(running_server, score_request, caplog):
    stub, _, _ = running_server
    with caplog.at_level(logging.INFO):
        stub.Score(
            score_request,
            timeout=2,
            metadata=(
                ("attempt-id", "a2000000-0000-4000-8000-000000000001"),
                ("trace-id", "unsafe value with spaces"),
                ("authorization", "never-log-this-token"),
            ),
        )
    event = json.loads(caplog.records[-1].message)
    assert event["application_id"] == score_request.application_id
    assert event["attempt_id"] == "a2000000-0000-4000-8000-000000000001"
    assert "trace_id" not in event
    assert "never-log-this-token" not in caplog.text
    assert "1200000000" not in caplog.text and "180000000" not in caplog.text


def test_invalid_config_exits_before_binding(tmp_path, monkeypatch, caplog):
    config = tmp_path / "broken.json"
    config.write_text('{"contains":"private configuration"}', encoding="utf-8")
    monkeypatch.setenv("SCORING_MODEL_PATH", str(config))

    def forbidden_bind(*_args, **_kwargs):
        pytest.fail("server must not bind with invalid model configuration")

    monkeypatch.setattr(grpc_service, "create_server", forbidden_bind)
    with pytest.raises(SystemExit) as error:
        grpc_service.serve()
    assert error.value.code == 1
    assert "private configuration" not in caplog.text
    assert "INVALID_CONFIGURATION" in caplog.text


def test_cli_stops_cleanly_on_sigterm():
    repository = Path(__file__).resolve().parents[3]
    with socket.socket() as reservation:
        reservation.bind(("127.0.0.1", 0))
        port = reservation.getsockname()[1]
    environment = os.environ.copy()
    environment.pop("SCORING_MODEL_PATH", None)
    environment["SCORING_ADDR"] = f"127.0.0.1:{port}"
    environment["PYTHONPATH"] = os.pathsep.join(
        [str(repository / "ml/scoring/src"), str(repository / "proto/gen/python")]
    )
    process = subprocess.Popen(
        [sys.executable, "-m", "smart_credit_scoring.grpc_service"],
        env=environment,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    try:
        with grpc.insecure_channel(environment["SCORING_ADDR"]) as channel:
            grpc.channel_ready_future(channel).result(timeout=5)
            response = health_pb2_grpc.HealthStub(channel).Check(
                health_pb2.HealthCheckRequest(service=grpc_service.SERVICE_NAME), timeout=2
            )
            assert response.status == health_pb2.HealthCheckResponse.SERVING
        process.send_signal(signal.SIGTERM)
        output, _ = process.communicate(timeout=7)
        assert process.returncode == 0
        assert '"outcome":"STOPPED"' in output
        assert "Traceback" not in output
    finally:
        if process.poll() is None:
            process.kill()
            process.communicate(timeout=3)
