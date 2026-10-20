.DEFAULT_GOAL := help

ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml
POSTGRES_PORT ?= 5432
BANK_DATABASE_URL ?= postgres://bank_user:$(BANK_DB_PASSWORD)@127.0.0.1:$(POSTGRES_PORT)/bank_db?sslmode=disable
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.24.1
BANK_MIGRATIONS := services/bank-service/migrations

.PHONY: help require-env infra-up infra-down infra-ps infra-logs bank-db-up bank-db-down bank-db-status

help:
	@echo 'Сначала: cp .env.example .env'
	@echo 'make infra-up         Запустить Postgres и Kafka, создать топики'
	@echo 'make infra-ps         Показать состояние контейнеров'
	@echo 'make infra-logs       Показать логи инфраструктуры'
	@echo 'make bank-db-up       Применить миграции bank_db'
	@echo 'make bank-db-down     Откатить миграцию bank_db'
	@echo 'make bank-db-status   Показать состояние миграций bank_db'
	@echo 'make infra-down       Остановить контейнеры, сохранив данные'

require-env:
	@test -f .env || { echo 'Сначала выполните: cp .env.example .env' >&2; exit 1; }

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

bank-db-up: require-env
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" up

bank-db-down: require-env
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" down

bank-db-status: require-env
	$(GOOSE) -dir $(BANK_MIGRATIONS) postgres "$(BANK_DATABASE_URL)" status
