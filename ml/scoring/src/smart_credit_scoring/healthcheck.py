"""Standard gRPC readiness probe with a bounded deadline."""

import argparse

import grpc
from grpc_health.v1 import health_pb2, health_pb2_grpc


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", default="127.0.0.1:50051")
    args = parser.parse_args()
    with grpc.insecure_channel(args.target) as channel:
        try:
            result = health_pb2_grpc.HealthStub(channel).Check(
                health_pb2.HealthCheckRequest(service="smartcredit.scoring.ScoringService"),
                timeout=2,
            )
        except grpc.RpcError:
            raise SystemExit(1) from None
        if result.status != health_pb2.HealthCheckResponse.SERVING:
            raise SystemExit(1)


if __name__ == "__main__":
    main()
