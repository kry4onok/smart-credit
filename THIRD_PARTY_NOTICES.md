# Third-party notices

Перечень сторонних компонентов, моделей, данных, сервисов и смоделированных интеграций проекта «Умный кредитный кабинет». Точные версии закреплены в `go.mod`/`go.sum`, `web/package-lock.json`, `ml/*/uv.lock`, `Makefile`, `scripts/proto-tools.json`, `deploy/docker-compose.yml` и `deploy/docker/*.Dockerfile`. Лицензии сторонних дистрибутивов действуют в их исходной редакции.

Платные API и внешние облачные сервисы не используются. Языковая модель — открытые веса, запускается локально через Ollama. Все данные синтетические.

## Qwen2.5 3B через Ollama

- Модель: `qwen2.5:3b` (локальные открытые веса; хранится в Ollama на компьютере, не включена в Docker-образ проекта).
- Правообладатель весов: Alibaba Cloud.
- Лицензия, которую показывает установленная модель: **Qwen Research License Agreement**, release date 19 September 2024. Она ограничивает использование некоммерческими исследовательскими и оценочными целями; для коммерческого применения требуется отдельное разрешение правообладателя.
- Полный текст лицензии установленной модели: `ollama show qwen2.5:3b --license`.
- Размеры `qwen2.5:1.5b` и `qwen2.5:7b` распространяются под Apache-2.0 и выбираются переменной `OLLAMA_MODEL` в `.env` без изменения кода.
- Модель не принимает и не меняет кредитные решения: она только формулирует тексты по фактам backend, а при недоступности используется детерминированный шаблон.

Перед заменой тега модели проверьте лицензию новой версии тем же способом и обновите этот файл.

## Ollama

| Компонент | Лицензия | Назначение |
|---|---|---|
| Ollama | MIT | Локальный runtime модели на хосте (`OLLAMA_URL`, по умолчанию `http://host.docker.internal:11434`). Не входит в compose и образы проекта. |

Настройка `LLM_PROVIDER=openai` в `ml/assistant/server.py` предназначена только для OpenAI-совместимого сервера, запущенного локально (`LLM_BASE_URL`); в поставке не используется. Ключи платных API проект не требует.

## Веб-интерфейс (`web/`)

Точные версии закреплены в `web/package-lock.json`.

| Компонент | Версия | Лицензия | Назначение |
|---|---|---|---|
| React, React DOM | 19.3.1 | MIT | UI |
| React Router | 7.18.4 | MIT | маршруты ролей |
| openapi-fetch | 0.17.0 | MIT | типизированные HTTP-вызовы |
| Golos Text (Paratype), через @fontsource | 5.3.0 | SIL OFL 1.1 | основной шрифт, хранится в сборке |
| Unbounded (NaN), через @fontsource | 5.3.0 | SIL OFL 1.1 | шрифт сумм и заголовков, хранится в сборке |
| Vite, @vitejs/plugin-react | 8.3.5, 6.1.2 | MIT | сборка (только разработка) |
| openapi-typescript | 7.13.0 | MIT | генерация TS-типов из OpenAPI (только разработка) |
| TypeScript | 5.9.3 | Apache-2.0 | проверка типов (только разработка) |
| nginx (образ `nginx:1.30.6-alpine`) | 1.30.6 | BSD-2-Clause | раздача статики и прокси `/api` |

Демо-режим web (`VITE_API_MOCK=true`) моделирует auth, client-service и bank-service
в браузере на синтетических данных; в production-сборку не входит.

## Go-модули

Прямые зависимости из `go.mod`; лицензии сверены с файлами `LICENSE` в кеше модулей. Транзитивные модули перечислены в `go.sum` и сохраняют свои лицензии.

| Модуль | Версия | Лицензия | Назначение |
|---|---|---|---|
| github.com/gabriel-vasile/mimetype | v1.4.15 | MIT | проверка содержимого загружаемых файлов |
| github.com/go-chi/chi/v5 | v5.5.0 | MIT | HTTP-маршрутизация |
| github.com/go-playground/validator/v10 | v10.30.5 | MIT | валидация конфигурации и DTO |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause | UUID |
| github.com/ilyakaznacheev/cleanenv | v1.5.0 | MIT | конфигурация из переменных окружения |
| github.com/jackc/pgx/v5 | v5.11.0 | MIT | драйвер PostgreSQL |
| github.com/oapi-codegen/runtime | v1.7.0 | Apache-2.0 | runtime сгенерированного OpenAPI-кода |
| github.com/twmb/franz-go | v1.22.2 | BSD-3-Clause | клиент Kafka |
| golang.org/x/crypto | v0.57.0 | BSD-3-Clause | Argon2id |
| google.golang.org/grpc | v1.78.0 | Apache-2.0 | gRPC-клиенты и сервер bureau-mocks |
| google.golang.org/protobuf | v1.36.12 | BSD-3-Clause | protobuf |
| gopkg.in/yaml.v3 | v3.0.1 | MIT и Apache-2.0 | чтение `seed/personas.yaml` |

## Инструменты разработки

Запускаются через `go run`/`uv` с закреплёнными версиями и в сборку сервисов не входят.

| Инструмент | Версия | Лицензия | Где используется |
|---|---|---|---|
| Go toolchain | 1.27.1 | BSD-3-Clause | сборка и тесты |
| goose (`github.com/pressly/goose/v3`) | v3.24.1 | MIT | миграции (`Makefile`, `GOOSE`) |
| oapi-codegen (`github.com/oapi-codegen/oapi-codegen/v2`) | v2.8.0 | Apache-2.0 | генерация Go из OpenAPI (`make gen-openapi`) |
| golangci-lint (`github.com/golangci/golangci-lint/v2`) | v2.15.0 | GPL-3.0 | линтеры (`make lint`); исполняется отдельно, не линкуется в код проекта |
| grpcio-tools (включает protoc) | 1.78.0 | Apache-2.0 | генерация protobuf (`scripts/generate_proto.py`) |
| protoc-gen-go | v1.36.11 | BSD-3-Clause | генерация Go protobuf |
| protoc-gen-go-grpc | v1.5.1 | Apache-2.0 | генерация Go gRPC |
| uv | 0.12.3 | MIT OR Apache-2.0 | окружения Python и образы ML-сервисов |
| Hatchling | 1.27.0 | MIT | сборка пакета scoring |
| pytest | 8.4.2 | MIT | тесты scoring |
| Hypothesis | 6.140.3 | MPL-2.0 | property-тесты scoring |
| Ruff | 0.14.0 | MIT | линтер Python |
| GitHub Actions: checkout, setup-go, setup-node, setup-uv, golangci-lint-action | v4–v7 | MIT | CI (`.github/workflows`) |

Buf в проекте не используется.

## Python-пакеты (`ml/`)

Версии и транзитивные пакеты закреплены в `ml/scoring/uv.lock` и `ml/assistant/uv.lock`.

| Пакет | Версия | Лицензия | Сервис |
|---|---|---|---|
| grpcio | 1.78.0 | Apache-2.0 | scoring, assistant |
| grpcio-health-checking | 1.78.0 | Apache-2.0 | scoring |
| protobuf | 6.33.5 | BSD-3-Clause | scoring, assistant |
| typing-extensions (транзитивно) | 4.16.0 | PSF-2.0 | scoring, assistant |

Транзитивные dev-пакеты scoring: attrs (MIT), colorama (BSD-3-Clause), iniconfig (MIT), packaging (Apache-2.0 OR BSD-2-Clause), pluggy (MIT), Pygments (BSD-2-Clause), setuptools (MIT), sortedcontainers (Apache-2.0). Лицензии указаны по метаданным проектов; в `uv.lock` они не хранятся.

## Образы контейнеров

| Образ | Лицензия основного ПО | Где |
|---|---|---|
| `postgres:16.15` | PostgreSQL License | compose, CI |
| `apache/kafka:3.9.2` | Apache-2.0 | compose |
| `nginx:1.30.6-alpine` | BSD-2-Clause | `gateway`, `web` |
| `docker.swagger.io/swaggerapi/swagger-ui:v5.33.1` | Apache-2.0 | `swagger-ui` |
| `golang:1.27.1-alpine` | BSD-3-Clause | стадия сборки Go-сервисов |
| `alpine:3.22` | пакеты под собственными лицензиями (BusyBox — GPL-2.0, ca-certificates — MPL-2.0) | runtime Go-сервисов |
| `node:22.12.0-alpine` | MIT | стадия сборки web |
| `python:3.12.12-slim-bookworm` | PSF-2.0; пакеты Debian под собственными лицензиями | runtime scoring и assistant |
| `ghcr.io/astral-sh/uv:0.12.3` | MIT OR Apache-2.0 | копирование `uv` в образы scoring и assistant |

## Модели и данные

- Скоринг: `monotone-scorecard-v1` (`ml/scoring/models/scorecard-v1.json`) и экспертная скоркарта `expert-cashflow-scorecard-v2` (`pkg/assessment`) — оригинальные артефакты команды, детерминированные правила без обучения. Внешние датасеты и предобученные веса для скоринга не используются; отдельной лицензии на скоркарты нет, действует политика лицензирования репозитория.
- Внешние датасеты не используются. Все данные синтетические и созданы командой: персоны `seed/personas.yaml`, `seed/scoring/personas.json`, `seed/profile-requirements/*`, демо-продукты и дела `services/bank-service/seeds/demo.sql`, сценарии `ml/assistant/scenarios.json`, каталог и fixtures Help `pkg/assistanthelp/`, демо-выписки `services/bank-service/internal/document/statement/testdata/example.txt` и `postman/fixtures/bank-statement.txt`, данные демо-режима `web/src/mock/`. ИНН, названия организаций, суммы и операции вымышлены.
- Единственные сторонние веса — Qwen2.5 (раздел выше), используются только для формулировки текстов.

## Смоделированные интеграции

Реальные государственные реестры, БКИ, банковские и платёжные системы не подключены. Все ответы детерминированы и помечены `is_mock=true`.

| Интеграция | Как смоделирована | Где в коде |
|---|---|---|
| Реестр компаний: ЕГРЮЛ/ЕГРИП, ФНС «Прозрачный бизнес» (статус, дата регистрации, ОКВЭД, руководитель, учредители) | gRPC `BureauService/GetCompanyProfile` по персонам | `services/bureau-mocks`, `proto/bureau/bureau.proto`, `seed/personas.yaml` |
| БКИ, кредитная история (долг, обязательства, просрочки за 12 месяцев) | gRPC `GetCreditHistory`; настроенные ошибки и таймауты для проверки деградации | `services/bureau-mocks` |
| Проверки допуска: банкротство, арбитражные дела, исполнительные производства (ФССП) | Источники `BANKRUPTCY`, `ARBITRATION`, `ENFORCEMENT` в `GetAssessmentSources` | `pkg/assessment/demo.go` |
| Связанные лица ООО (руководитель, владелец): банкротство и исполнительные производства | Источники с ролями `DIRECTOR`, `OWNER` | `pkg/assessment/demo.go` |
| ГИР БО (бухгалтерская отчётность ООО за прошлый год) | Сведение `demo-girbo:<ИНН>`, закрывающее требование `BUSINESS_RESULT` | `pkg/assessment/demo.go` |
| Банковские выписки и обороты по счёту | 12 месяцев синтетического денежного потока `demo-bank-ledger:<ИНН>`; загруженные клиентом файлы хранятся как демовложения (`is_mock=true`, SHA-256) без извлечения данных; парсер демо-формата TXT используется только в тестах | `pkg/assessment/demo.go`, `services/client-service`, `services/bank-service/internal/document/statement` |
| Электронная подпись договора | Кнопка «Подписать» отправляет событие `ContractSigned`; удостоверяющий центр и ЭП не используются | `services/client-service`, `services/bank-service/internal/repository/receive/signing.go` |
| Выдача кредита | Worker демо-выдачи переводит дело `SIGNING → DISBURSED` через 3 секунды; платёжные системы не вызываются | `services/bank-service/internal/repository/disbursement` |
| Backend в браузере | Демо-режим web `VITE_API_MOCK=true` | `web/src/mock/` |
| Сервисы ФНС для клиента: ЛК ИП (`lkip2.nalog.ru`), ЛК юрлица (`lkul.nalog.ru`), ГИР БО (`bo.nalog.gov.ru`), выписка ЕГРЮЛ/ЕГРИП (`egrul.nalog.ru`), «Прозрачный бизнес» (`pb.nalog.ru`) | Только ссылки, открываются в новой вкладке; API этих сервисов не вызываются | `web/src/lib/external.ts` |
