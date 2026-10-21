.DEFAULT_GOAL := help

ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml
POSTGRES_PORT ?= 5432
SWAGGER_PORT ?= 8081
BANK_DATABASE_URL ?= postgres://bank_user:$(BANK_DB_PASSWORD)@127.0.0.1:$(POSTGRES_PORT)/bank_db?sslmode=disable
CLIENT_DATABASE_URL ?= postgres://client_user:$(CLIENT_DB_PASSWORD)@127.0.0.1:$(POSTGRES_PORT)/client_db?sslmode=disable
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.24.1
BANK_MIGRATIONS := services/bank-service/migrations
CLIENT_MIGRATIONS := services/client-service/migrations
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
OPENAPI_OUTPUT_DIRS := \
	pkg/api/common \
	services/auth/internal/generated \
	services/client-service/internal/generated \
	services/bank-service/internal/generated

.PHONY: help require-env gen gen-openapi lint infra-up infra-down infra-ps infra-logs bank-run bank-db-up bank-db-down bank-db-status

help:
	@echo 'Сначала: cp .env.example .env'
	@echo 'make gen              Сгенерировать код из всех контрактов'
	@echo 'make gen-openapi      Сгенерировать Go-код из OpenAPI'
	@echo 'make lint             Проверить Go-код линтерами и форматтерами'
	@echo 'make docs-up          Запустить Swagger UI для всех API'
	@echo 'make docs-down        Остановить и удалить Swagger UI'
	@echo 'make docs-logs        Показать логи Swagger UI'
	@echo 'make infra-up         Запустить Postgres и Kafka, создать топики'
	@echo 'make infra-ps         Показать состояние контейнеров'
	@echo 'make infra-logs       Показать логи инфраструктуры'
	@echo 'make bank-run         Запустить bank-service локально'
	@echo 'make bank-db-up       Применить миграции bank_db'
	@echo 'make bank-db-down     Откатить миграцию bank_db'
	@echo 'make bank-db-status   Показать состояние миграций bank_db'
	@echo 'make client-db-up     Применить миграции client_db'
	@echo 'make client-db-down   Откатить миграцию client_db'
	@echo 'make client-db-status Показать состояние миграций client_db'
	@echo 'make infra-down       Остановить контейнеры, сохранив данные'

require-env:
	@test -f .env || { echo 'Сначала выполните: cp .env.example .env' >&2; exit 1; }

gen: gen-openapi

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

bank-db-up: require-env
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" up

bank-db-down: require-env
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" down

bank-db-status: require-env
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" status

client-db-up: require-env
	$(GOOSE) -dir $(CLIENT_MIGRATIONS) postgres "$(CLIENT_DATABASE_URL)" up

client-db-down: require-env
	$(GOOSE) -dir $(CLIENT_MIGRATIONS) postgres "$(CLIENT_DATABASE_URL)" down

client-db-status: require-env
	$(GOOSE) -dir $(CLIENT_MIGRATIONS) postgres "$(CLIENT_DATABASE_URL)" status
