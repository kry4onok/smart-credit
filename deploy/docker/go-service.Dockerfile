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

FROM alpine:3.22

RUN apk add --no-cache ca-certificates && \
    addgroup -S app && \
    adduser -S -G app app && \
    mkdir -p /var/lib/smart-credit/documents && \
    chown -R app:app /var/lib/smart-credit

COPY --from=builder /out/service /usr/local/bin/service

USER app

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/service"]
