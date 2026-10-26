FROM ghcr.io/astral-sh/uv:0.12.3@sha256:2d890623d310b57771ce840f0da5eed5fc6d657da05ffaa45d82797b53fa3abc AS uv
FROM python:3.12.12-slim-bookworm@sha256:593bd06efe90efa80dc4eee3948be7c0fde4134606dd40d8dd8dbcade98e669c

COPY --from=uv /uv /usr/local/bin/uv
ENV UV_PYTHON_DOWNLOADS=never \
    UV_LINK_MODE=copy \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/app/proto/gen/python \
    ASSISTANT_ADDR=0.0.0.0:50052
WORKDIR /app/ml/assistant
COPY ml/assistant/pyproject.toml ml/assistant/uv.lock ml/assistant/.python-version ./
RUN uv sync --locked --no-dev --no-editable --no-cache
COPY ml/assistant/server.py ./server.py
COPY ml/assistant/help ./help
COPY pkg/assistanthelp/catalog.json /app/pkg/assistanthelp/catalog.json
COPY ml/assistant/scenarios.json ./scenarios.json
COPY scripts/assistant_smoke.py /app/scripts/assistant_smoke.py
COPY proto/gen/python /app/proto/gen/python
USER 10001:10001
EXPOSE 50052
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=5 \
    CMD [".venv/bin/python", "-c", "import socket; socket.create_connection(('127.0.0.1', 50052), 2).close()"]
CMD [".venv/bin/python", "server.py"]
