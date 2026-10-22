FROM ghcr.io/astral-sh/uv:0.12.3@sha256:2d890623d310b57771ce840f0da5eed5fc6d657da05ffaa45d82797b53fa3abc AS uv
FROM python:3.12.12-slim-bookworm@sha256:593bd06efe90efa80dc4eee3948be7c0fde4134606dd40d8dd8dbcade98e669c

COPY --from=uv /uv /usr/local/bin/uv
ENV UV_PYTHON_DOWNLOADS=never \
    UV_LINK_MODE=copy \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/app/proto/gen/python \
    SCORING_ADDR=0.0.0.0:50051
WORKDIR /app/ml/scoring
COPY ml/scoring/pyproject.toml ml/scoring/uv.lock ml/scoring/.python-version ./
COPY ml/scoring/src ./src
COPY ml/scoring/models ./models
RUN uv sync --locked --no-dev --no-editable --no-cache
COPY proto/gen/python /app/proto/gen/python
USER 10001:10001
EXPOSE 50051
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=5 \
    CMD [".venv/bin/python", "-m", "smart_credit_scoring.healthcheck"]
CMD [".venv/bin/smart-credit-scoring"]
