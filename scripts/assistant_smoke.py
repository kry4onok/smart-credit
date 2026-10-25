"""Прямой gRPC-прогон снимка семи банковских demo-сценариев без базы данных."""

import argparse
import json
import time
from pathlib import Path

import grpc

from proto.assistant import assistant_pb2, assistant_pb2_grpc


SCENARIOS = Path(__file__).resolve().parents[1] / "ml/assistant/scenarios.json"


def check_summary(text: str, fields: dict) -> None:
    for field in ("product_name", "grade"):
        if fields.get(field):
            assert fields[field] in text, f"нет {field}: {text}"
    for field in ("purpose", "status"):
        expected = fields.get(f"{field}_description") or fields.get(field)
        if expected:
            assert expected in text, f"нет {field}: {text}"
    for details_key, codes_key in (
        ("reason_facts", "reason_codes"),
        ("triggered_rule_facts", "triggered_rule_codes"),
    ):
        details = fields.get(details_key, [])
        expected = [item["description"] or item["code"] for item in details] if details else fields.get(codes_key, [])
        if details_key == "triggered_rule_facts":
            expected = [
                "Нужен ручной разбор" if item["code"] == "REVIEW_REQUIRED"
                and item["description"] == "Необходим ручной разбор; конкретная причина этим кодом не указана"
                else item["description"] or item["code"]
                for item in details
            ] if details else expected
        for value in expected:
            assert value in text, f"нет {value}: {text}"
    if not fields.get("grade"):
        assert "Грейд" not in text and "grade" not in text.casefold(), f"выдуман грейд: {text}"
    if not fields.get("reason_codes") and not fields.get("reason_facts"):
        assert "Факторы скоринга" not in text, f"выдуманы причины: {text}"
    if not fields.get("triggered_rule_codes") and not fields.get("triggered_rule_facts"):
        assert "Правила банка" not in text, f"выдуманы правила: {text}"


def check_version(version: str, expected: str | None) -> None:
    if expected:
        assert version == expected, f"ожидалась версия {expected}, получена {version}"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", default="127.0.0.1:50052")
    parser.add_argument("--timeout", type=float, default=5, help="Дедлайн каждого вызова, как у bank-service")
    parser.add_argument("--expect-version", help="Например: qwen2.5:3b или template")
    args = parser.parse_args()
    scenarios = json.loads(SCENARIOS.read_text(encoding="utf-8"))
    assert len(scenarios) == 7, "ожидалось семь банковских demo-сценариев"
    calls = 0
    templates = 0

    with grpc.insecure_channel(args.target) as channel:
        grpc.channel_ready_future(channel).result(timeout=5)
        assistant = assistant_pb2_grpc.AssistantServiceStub(channel)
        for scenario in scenarios:
            label = scenario["status"] + (f" ({scenario['note']})" if scenario.get("note") else "")
            fields = scenario["summary"]
            started = time.perf_counter()
            try:
                summary = assistant.SummarizeCase(
                    assistant_pb2.SummarizeCaseRequest(**fields), timeout=args.timeout
                )
            except grpc.RpcError as exc:
                raise SystemExit(f"#{scenario['number']} SummarizeCase: {exc.code().name} после {time.perf_counter() - started:.1f} с") from exc
            check_summary(summary.text, fields)
            check_version(summary.model_version, args.expect_version)
            print(f"#{scenario['number']} {label} SummarizeCase [{summary.model_version}, {time.perf_counter() - started:.1f} с]: {summary.text}", flush=True)
            calls += 1
            templates += summary.model_version == "template"

            if explanation_fields := scenario.get("explanation"):
                started = time.perf_counter()
                try:
                    explanation = assistant.ExplainDecision(
                        assistant_pb2.ExplainDecisionRequest(**explanation_fields), timeout=args.timeout
                    )
                except grpc.RpcError as exc:
                    raise SystemExit(f"#{scenario['number']} ExplainDecision: {exc.code().name} после {time.perf_counter() - started:.1f} с") from exc
                check_version(explanation.model_version, args.expect_version)
                assert explanation_fields["analyst_public_reason"] in explanation.text
                assert explanation_fields["product_name"] in explanation.text
                for fragment in scenario.get("explanation_expected", []):
                    assert fragment in explanation.text, f"нет {fragment}: {explanation.text}"
                print(f"#{scenario['number']} {label} ExplainDecision [{explanation.model_version}, {time.perf_counter() - started:.1f} с]: {explanation.text}", flush=True)
                calls += 1
                templates += explanation.model_version == "template"

    print(f"PASS: {len(scenarios)} сценариев, {calls} вызовов; модель: {calls - templates}, шаблон: {templates}")


if __name__ == "__main__":
    main()
