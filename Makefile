.DEFAULT_GOAL := help

ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml
POSTGRES_PORT ?= 5432
SCORING_PORT ?= 50051
SWAGGER_PORT ?= 8081
GATEWAY_PORT ?= 8000
BANK_DATABASE_URL ?= postgres://bank_user:$(BANK_DB_PASSWORD)@127.0.0.1:$(POSTGRES_PORT)/bank_db?sslmode=disable
CLIENT_DATABASE_URL ?= postgres://client_user:$(CLIENT_DB_PASSWORD)@127.0.0.1:$(POSTGRES_PORT)/client_db?sslmode=disable
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.24.1
BANK_MIGRATIONS := services/bank-service/migrations
BANK_DEMO_SEED := services/bank-service/seeds/demo.sql
CLIENT_MIGRATIONS := services/client-service/migrations
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
OPENAPI_OUTPUT_DIRS := \
	pkg/api/common \
	services/auth/internal/generated \
	services/client-service/internal/generated \
	services/bank-service/internal/generated

.PHONY: help require-env gen gen-openapi lint docs-up docs-down docs-logs gateway-up gateway-down gateway-reload gateway-logs infra-up infra-down infra-ps infra-logs bank-run bank-up bank-down bank-logs bank-db-up bank-db-down bank-db-status bank-demo-seed
.PHONY: client-up client-down client-logs

help:
	@echo 'Сначала: cp .env.example .env'
	@echo 'make gen              Сгенерировать код из всех контрактов'
	@echo 'make gen-openapi      Сгенерировать Go-код из OpenAPI'
	@echo 'make gen-proto        Сгенерировать Go/Python из protobuf'
	@echo 'make scoring-sync     Установить Python-окружение scoring'
	@echo 'make scoring-test     Проверить модель и gRPC'
	@echo 'make scoring-up       Собрать и запустить scoring в Docker'
	@echo 'make scoring-smoke    Проверить scoring из Python'
	@echo 'make scoring-smoke-go Проверить scoring из Go'
	@echo 'make bank-scoring-smoke Проверить банковский adapter со scoring'
	@echo 'make lint             Проверить Go-код линтерами и форматтерами'
	@echo 'make docs-up          Запустить Swagger UI для всех API'
	@echo 'make docs-down        Остановить и удалить Swagger UI'
	@echo 'make docs-logs        Показать логи Swagger UI'
	@echo 'make gateway-up       Запустить nginx gateway и Swagger UI'
	@echo 'make gateway-down     Остановить и удалить gateway и Swagger UI'
	@echo 'make gateway-reload   Проверить и перечитать конфигурацию nginx'
	@echo 'make gateway-logs     Показать логи gateway'
	@echo 'make infra-up         Запустить Postgres и Kafka, создать топики'
	@echo 'make infra-ps         Показать состояние контейнеров'
	@echo 'make infra-logs       Показать логи инфраструктуры'
	@echo 'make bank-run         Запустить bank-service локально'
	@echo 'make bank-up          Собрать и запустить bank-service в Docker'
	@echo 'make bank-down        Остановить и удалить контейнер bank-service'
	@echo 'make bank-logs        Показать логи bank-service'
	@echo 'make bank-db-up       Применить миграции bank_db'
	@echo 'make bank-db-down     Откатить миграцию bank_db'
	@echo 'make bank-db-status   Показать состояние миграций bank_db'
	@echo 'make bank-demo-seed   Загрузить демоданные в bank_db'
	@echo 'make client-db-up     Применить миграции client_db'
	@echo 'make client-up        Собрать и запустить client-service в Docker'
	@echo 'make client-down      Остановить и удалить контейнер client-service'
	@echo 'make client-logs      Показать логи client-service'
	@echo 'make client-db-down   Откатить миграцию client_db'
	@echo 'make client-db-status Показать состояние миграций client_db'
	@echo 'make infra-down       Остановить контейнеры, сохранив данные'

require-env:
	@test -f .env || { echo 'Сначала выполните: cp .env.example .env' >&2; exit 1; }

gen: gen-openapi gen-proto

.PHONY: gen-proto gen-proto-check scoring-sync
scoring-sync:
	uv sync --project ml/scoring --locked

gen-proto:
	uv run --no-project --python 3.12.12 scripts/generate_proto.py

gen-proto-check:
	uv run --no-project --python 3.12.12 scripts/generate_proto.py --check

.PHONY: scoring-test scoring-lint scoring-run scoring-up scoring-down scoring-smoke scoring-smoke-go bank-scoring-smoke scoring-evaluate
SCORING_TARGET ?= 127.0.0.1:$(SCORING_PORT)
SCORING_PY := uv run --project ml/scoring --locked

scoring-test:
	$(SCORING_PY) pytest ml/scoring/tests

scoring-lint:
	$(SCORING_PY) ruff check --config ml/scoring/pyproject.toml ml/scoring/src ml/scoring/tests scripts/generate_proto.py scripts/scoring_smoke.py scripts/evaluate_scoring.py
	$(SCORING_PY) ruff format --check --config ml/scoring/pyproject.toml ml/scoring/src ml/scoring/tests scripts/generate_proto.py scripts/scoring_smoke.py scripts/evaluate_scoring.py

scoring-run:
	PYTHONPATH=proto/gen/python $(SCORING_PY) smart-credit-scoring

scoring-up: require-env
	$(COMPOSE) up -d --build --wait scoring

scoring-down: require-env
	$(COMPOSE) rm -sf scoring

scoring-smoke:
	PYTHONPATH=proto/gen/python $(SCORING_PY) python scripts/scoring_smoke.py --target $(SCORING_TARGET)

scoring-smoke-go:
	go run ./cmd/scoring-smoke --target $(SCORING_TARGET)

bank-scoring-smoke:
	BANK_SCORING_TARGET=$(SCORING_TARGET) go test ./services/bank-service/internal/client/scoring -run '^TestLiveScoringAdapter$$' -count=1 -v

scoring-evaluate:
	PYTHONPATH=proto/gen/python $(SCORING_PY) python scripts/evaluate_scoring.py --target $(SCORING_TARGET) --output /tmp/smart-credit-scoring-evaluation.json

gen-openapi:
	@mkdir -p $(OPENAPI_OUTPUT_DIRS)
	$(OAPI_CODEGEN) -config api/codegen/common.yaml api/common.yaml
	$(OAPI_CODEGEN) -config api/codegen/auth.yaml api/auth.yaml
	$(OAPI_CODEGEN) -config api/codegen/client.yaml api/client.yaml
	$(OAPI_CODEGEN) -config api/codegen/bank.yaml api/bank.yaml

lint:
	$(GOLANGCI_LINT) run ./...

docs-up: require-env
	$(COMPOSE) up -d --wait swagger-ui
	@echo 'Swagger UI: http://localhost:$(SWAGGER_PORT)'

docs-down: require-env
	$(COMPOSE) rm -sf swagger-ui

docs-logs: require-env
	$(COMPOSE) logs -f swagger-ui

gateway-up: require-env
	$(COMPOSE) up -d --wait swagger-ui gateway
	@echo 'Swagger UI with API gateway: http://localhost:$(GATEWAY_PORT)/docs/'

gateway-down: require-env
	$(COMPOSE) rm -sf gateway swagger-ui

gateway-reload: require-env
	$(COMPOSE) exec gateway nginx -t
	$(COMPOSE) exec gateway nginx -s reload

gateway-logs: require-env
	$(COMPOSE) logs -f gateway

infra-up: require-env
	$(COMPOSE) up -d --wait postgres kafka
	$(COMPOSE) exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:19092 --create --if-not-exists --topic client.events --partitions 1 --replication-factor 1
	$(COMPOSE) exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:19092 --create --if-not-exists --topic bank.events --partitions 1 --replication-factor 1

infra-down: require-env
	$(COMPOSE) down

infra-ps: require-env
	$(COMPOSE) ps

infra-logs: require-env
	$(COMPOSE) logs -f postgres kafka

bank-run: require-env
	go run ./services/bank-service/cmd/service

bank-up: require-env
	$(COMPOSE) up -d --wait postgres
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" up
	$(COMPOSE) up -d --build --wait bank-service

bank-down: require-env
	$(COMPOSE) rm -sf bank-service

bank-logs: require-env
	$(COMPOSE) logs -f bank-service

bank-db-up: require-env
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" up

bank-db-down: require-env
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" down

bank-db-status: require-env
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" status

bank-demo-seed: require-env
	$(COMPOSE) exec -T postgres sh -c 'PGPASSWORD="$$BANK_DB_PASSWORD" psql -h 127.0.0.1 -U bank_user -d bank_db -v ON_ERROR_STOP=1' < $(BANK_DEMO_SEED)

client-up: require-env
	$(COMPOSE) up -d --wait postgres
	$(GOOSE) -dir $(CLIENT_MIGRATIONS) postgres "$(CLIENT_DATABASE_URL)" up
	$(COMPOSE) up -d --build --wait client-service

client-down: require-env
	$(COMPOSE) rm -sf client-service

client-logs: require-env
	$(COMPOSE) logs -f client-service

client-db-up: require-env
	$(GOOSE) -dir $(CLIENT_MIGRATIONS) postgres "$(CLIENT_DATABASE_URL)" up

client-db-down: require-env
	$(GOOSE) -dir $(CLIENT_MIGRATIONS) postgres "$(CLIENT_DATABASE_URL)" down

client-db-status: require-env
	$(GOOSE) -dir $(CLIENT_MIGRATIONS) postgres "$(CLIENT_DATABASE_URL)" status
