"""Проверка gRPC assistant без банковского сервиса и базы данных."""

import argparse

import grpc

from proto.assistant import assistant_pb2, assistant_pb2_grpc


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--target", default="127.0.0.1:50052")
    parser.add_argument("--expect-version", help="Например: qwen2.5:3b или template")
    args = parser.parse_args()

    with grpc.insecure_channel(args.target) as channel:
        grpc.channel_ready_future(channel).result(timeout=5)
        assistant = assistant_pb2_grpc.AssistantServiceStub(channel)
        summary = assistant.SummarizeCase(
            assistant_pb2.SummarizeCaseRequest(
                application_id="demo-application",
                product_name="Потребительский кредит",
                purpose="ремонт квартиры",
                grade="B",
            ),
            timeout=35,
        )
        explanation = assistant.ExplainDecision(
            assistant_pb2.ExplainDecisionRequest(
                application_id="demo-application",
                outcome="REJECT",
                product_name="Потребительский кредит",
                analyst_public_reason="Подтверждённого дохода недостаточно для запрошенной суммы.",
            ),
            timeout=35,
        )

    assert "ремонт квартиры" in summary.text and "B" in summary.text
    assert "Правила не применялись" not in summary.text
    assert "решение об отказе" in explanation.text
    assert "Подтверждённого дохода недостаточно для запрошенной суммы" in explanation.text
    assert "повторно" not in explanation.text
    if args.expect_version:
        assert summary.model_version == args.expect_version, summary.model_version
        assert explanation.model_version == args.expect_version, explanation.model_version

    print(f"SummarizeCase [{summary.model_version}]: {summary.text}")
    print(f"ExplainDecision [{explanation.model_version}]: {explanation.text}")


if __name__ == "__main__":
    main()
