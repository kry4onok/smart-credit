FROM golang:1.27.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE_PATH
RUN test -n "${SERVICE_PATH}" && \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/service \
      "./${SERVICE_PATH}/cmd/service"

FROM builder AS auth-keygen-builder

RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/keygen \
      ./services/auth/cmd/keygen

FROM alpine:3.22 AS runtime-base

RUN apk add --no-cache ca-certificates && \
    addgroup -S app && \
    adduser -S -G app app && \
    mkdir -p /var/lib/smart-credit/documents && \
    chown -R app:app /var/lib/smart-credit

FROM runtime-base AS auth

RUN mkdir -p /var/lib/smart-credit/auth && \
    chown app:app /var/lib/smart-credit/auth

COPY --from=builder /out/service /usr/local/bin/service
COPY --from=auth-keygen-builder /out/keygen /usr/local/bin/keygen

USER app

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/service"]

# Keep the generic service as the default target for existing Go services.
FROM runtime-base AS service

COPY --from=builder /out/service /usr/local/bin/service

USER app

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/service"]
