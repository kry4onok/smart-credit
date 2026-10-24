"""Прямой gRPC-прогон снимка семи банковских demo-сценариев без базы данных."""

import argparse
import json
import time
from pathlib import Path

import grpc

from proto.assistant import assistant_pb2, assistant_pb2_grpc


SCENARIOS = Path(__file__).resolve().parents[1] / "ml/assistant/scenarios.json"


def check_summary(text: str, fields: dict) -> None:
    for field in ("product_name", "purpose", "grade"):
        if fields.get(field):
            assert fields[field] in text, f"нет {field}: {text}"
    for field in ("reason_codes", "triggered_rule_codes"):
        for code in fields.get(field, []):
            assert code in text, f"нет {code}: {text}"
    if not fields.get("grade"):
        assert "Грейд" not in text and "grade" not in text.casefold(), f"выдуман грейд: {text}"
    if not fields.get("reason_codes"):
        assert "Коды причин" not in text, f"выдуманы причины: {text}"
    if not fields.get("triggered_rule_codes"):
        assert "Коды сработавших правил" not in text, f"выдуманы правила: {text}"


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
                print(f"#{scenario['number']} {label} ExplainDecision [{explanation.model_version}, {time.perf_counter() - started:.1f} с]: {explanation.text}", flush=True)
                calls += 1
                templates += explanation.model_version == "template"

    print(f"PASS: {len(scenarios)} сценариев, {calls} вызовов; модель: {calls - templates}, шаблон: {templates}")


if __name__ == "__main__":
    main()
