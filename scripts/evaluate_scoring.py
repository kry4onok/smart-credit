#!/usr/bin/env python3
"""Measure fixture agreement and real RPC latency; this is not default prediction validation."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import sys
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import grpc
from scoring_smoke import (
    DEFAULT_CONFIG,
    DEFAULT_FIXTURES,
    ContractError,
    check_health,
    load_cases,
    make_request,
    run_cases,
    verify_response,
)

from proto.scoring import scoring_pb2_grpc


def percentile(values: list[float], fraction: float) -> float:
    """Nearest-rank percentile; works for short, nonempty runs without interpolation."""
    ordered = sorted(values)
    return ordered[max(0, math.ceil(len(ordered) * fraction) - 1)]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", default="127.0.0.1:50051")
    parser.add_argument("--fixtures", type=Path, default=DEFAULT_FIXTURES)
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    parser.add_argument("--iterations", type=int, default=200)
    parser.add_argument("--concurrency", type=int, default=10)
    parser.add_argument("--output", type=Path)
    arguments = parser.parse_args()
    if not 1 <= arguments.iterations <= 10000 or not 1 <= arguments.concurrency <= 10:
        parser.error("iterations must be 1..10000 and concurrency 1..10")
    try:
        cases = load_cases(arguments.fixtures)
        checksum = hashlib.sha256(arguments.config.read_bytes()).hexdigest()
        case = next(item for item in cases if item["name"] == "risk_a")
        with grpc.insecure_channel(arguments.target) as channel:
            check_health(channel)
            results = run_cases(channel, cases, checksum)
            stub = scoring_pb2_grpc.ScoringServiceStub(channel)
            # Warm up transport, server, and workers before timing.
            for _ in range(10):
                verify_response(case, stub.Score(make_request(case), timeout=2), checksum)

            def invoke(_: int) -> float:
                request = make_request(case)
                start = time.perf_counter()
                response = stub.Score(request, timeout=2)
                elapsed = (time.perf_counter() - start) * 1000
                verify_response(case, response, checksum)
                return elapsed

            started = time.perf_counter()
            with ThreadPoolExecutor(max_workers=arguments.concurrency) as pool:
                durations = list(pool.map(invoke, range(arguments.iterations)))
            elapsed_seconds = time.perf_counter() - started
        report = {
            "model_version": "monotone-scorecard-v1",
            "config_checksum": checksum,
            "fixture_results": results,
            "fixture_passed": len(results),
            "benchmark": {
                "case": "risk_a",
                "iterations": len(durations),
                "concurrency": arguments.concurrency,
                "warmup_requests": 10,
                "rpc_timeout_seconds": 2,
                "latency_ms": {
                    "p50": round(percentile(durations, 0.5), 3),
                    "p95": round(percentile(durations, 0.95), 3),
                    "p99": round(percentile(durations, 0.99), 3),
                    "max": round(max(durations), 3),
                },
                "requests_per_second": round(len(durations) / elapsed_seconds, 2),
                "server_rss_bytes": None,
                "notes": "Client-observed RPC latency; server RSS is not measured by this client. "
                "Synthetic fixture agreement does not measure real default prediction.",
            },
        }
        output = json.dumps(report, ensure_ascii=False, indent=2) + "\n"
        if arguments.output:
            arguments.output.write_text(output, encoding="utf-8")
        print(output, end="")
        return 0
    except grpc.RpcError as error:
        print(f"Evaluation RPC failed: {error.code().name}", file=sys.stderr)
    except (ContractError, OSError, KeyError, ValueError, StopIteration) as error:
        print(f"Evaluation failed: {error}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
