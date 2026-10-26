import argparse
import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

import grpc
from google.protobuf.json_format import ParseDict
from proto.assistant import help_pb2, help_pb2_grpc


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--target", default="127.0.0.1:50052")
    parser.add_argument("--fixture", default="ml/assistant/tests/fixtures/help-smoke.json")
    parser.add_argument("--expect-source", choices=("model", "template"), default="template")
    args = parser.parse_args()
    payload = json.loads(Path(args.fixture).read_text(encoding="utf-8"))
    request = ParseDict(payload, help_pb2.HelpRequest())
    now = datetime.now(timezone.utc)
    request.context.generated_at.FromDatetime(now - timedelta(seconds=2))
    request.context.valid_until.FromDatetime(now + timedelta(seconds=58))
    with grpc.insecure_channel(args.target) as channel:
        response = help_pb2_grpc.AssistantHelpServiceStub(channel).Help(request, timeout=5)
    expected = help_pb2.RESPONSE_SOURCE_MODEL if args.expect_source == "model" else help_pb2.RESPONSE_SOURCE_TEMPLATE
    if response.source != expected or response.catalog_version != request.context.catalog_version or response.context_id != request.context.context_id or not response.blocks or not response.plan.status:
        raise SystemExit("Help smoke failed")
    print(json.dumps({"source": help_pb2.ResponseSource.Name(response.source), "status": help_pb2.AnswerStatus.Name(response.status), "fallback_reason": help_pb2.FallbackReason.Name(response.fallback_reason), "catalog_version": response.catalog_version, "model_version": response.model_version}, ensure_ascii=False))


if __name__ == "__main__":
    main()
