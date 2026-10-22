# Scoring dependency and artifact notices

The scorecard configuration and synthetic personas are original project artifacts.
No external dataset, pretrained weights, proprietary API or model download is used.
There is no separate license grant for the scorecard; the repository's licensing
policy applies.

Direct runtime dependencies (versions and transitive packages are in `ml/scoring/uv.lock`):

| Package | License | Source |
|---|---|---|
| grpcio 1.78.0 | Apache-2.0 | https://github.com/grpc/grpc |
| grpcio-health-checking 1.78.0 | Apache-2.0 | https://github.com/grpc/grpc |
| protobuf 6.33.5 | BSD-3-Clause | https://github.com/protocolbuffers/protobuf |

Go generated clients use gRPC-Go (Apache-2.0) and protobuf-go (BSD-3-Clause).
Development tools include grpcio-tools (Apache-2.0), pytest (MIT), Hypothesis
(MPL-2.0), Ruff (MIT), Hatchling (MIT), and uv (MIT OR Apache-2.0).
The container uses the official Python image and uv; upstream distributions retain
their own licenses. Generated headers identify their respective generators.
