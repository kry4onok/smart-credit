# Сценарии проверки

Сводный перечень автоматических и ручных проверок. Команды выполняются из корня репозитория после `cp .env.example .env` (если `.env` ещё нет). Подробности оценки v2 и Help — в [ASSESSMENT_RUNBOOK.md](ASSESSMENT_RUNBOOK.md), статусы и поток событий — в [ARCHITECTURE.md](ARCHITECTURE.md).

## 1. Статические проверки и контракты

| Проверка | Команда | Ожидаемый результат |
|---|---|---|
| Линтеры и форматирование Go | `make lint` | golangci-lint v2.15.0 без замечаний. |
| Сгенерированный protobuf актуален | `make gen-proto-check` | Go/Python-код в `proto/gen` совпадает с `proto/*/*.proto`. |
| Сиды профилей и моки согласованы | `make profile-seed-check` | Проходят `seed/profile-requirements`, `bureau-mocks/internal/persona`, `bureau-mocks/internal/grpcserver`. |
| Python-линтер scoring | `make scoring-lint` | `ruff check` и `ruff format --check` без замечаний. |
| Типы web | `npm --prefix web run typecheck` | `tsc -b` без ошибок; после изменения `api/*.yaml` сначала `make web-gen`. |

## 2. Unit- и интеграционные тесты Go

```sh
go test -race ./...
```

Без переменных БД выполняются unit-тесты, SQL-тесты пропускаются. Для SQL-тестов поднимите `make infra-up`, примените миграции (`make auth-db-up client-db-up bank-db-up`) и задайте выделенные тестовые базы:

```sh
export BANK_DATABASE_URL=...          # все миграции bank применены
export CLIENT_TEST_DATABASE_URL=...   # тесты создают отдельные схемы
export AUTH_TEST_DATABASE_URL=...
make auth-test client-test            # race detector, отказываются запускаться без переменных
go test -race ./services/bank-service/...
```

Не направляйте тесты на рабочую БД. Точечные цели Makefile для bank_db:

| Цель | Что проверяет | Ожидаемый результат |
|---|---|---|
| `make bank-outbox-test` | Relay outbox: порядок `aggregate_version`, повтор после истёкшего lease | Событие более поздней версии не публикуется раньше предыдущей. |
| `make bank-decision-test` | Решение аналитика: оферта, аудит, outbox, replay ключа; второй тест вызывает напрямую сохранённую функцию `complete` автоматического решения, которую worker v2 не использует | Одна запись решения, одна оферта, одно событие на повтор с тем же ключом. |
| `make bank-audit-test` | История дела | Стабильный порядок при равном времени, внутренние детали не раскрываются. |
| `make bank-catalog-test` | Публикация каталога | Полный снимок публикуется только при изменении `content_hash`. |
| `make bank-offer-test` | `OfferAccepted`/`OfferDeclined` | Атомарное применение и дедупликация по `processed_events`. |
| `make bank-cancel-test` | `ApplicationCancelled` | Дело `CANCELLED`, открытый запрос закрыт, оферта `DECLINED`, повтор игнорируется. |
| `make bank-signing-test` | `ContractSigned` и демо-выдача | `OFFER_ACCEPTED → SIGNING → DISBURSED`, ровно один `DEMO_DISBURSED`. |
| `make bank-offer-expiry-test` | Worker истечения оферт | Истечение один раз; ответ клиента до истечения выигрывает; заблокированное дело пропускается. |
| `make auth-db-test` | Миграции auth в изолированной схеме | Up/down без ошибок. |
| `make auth-dev-seed-test` | Seed учёток ANALYST/ADMIN/CLIENT | Учётки создаются идемпотентно. |
| `make bank-scoring-smoke` | Go-adapter к запущенному scoring (`TestLiveScoringAdapter`) | Ответ проходит проверку контракта v1. |

Ключевые unit-пакеты без БД: `pkg/assessment` (скоркарта v2, кандидаты, стресс, допуск), `pkg/creditcalc` (аннуитет), `pkg/requirements`, `pkg/assistanthelp` (рендер Help и паритет с Python по `pkg/assistanthelp/testdata/parity.json`), `services/bank-service/internal/document/statement` (парсер демо-выписки).

## 3. Python

| Проверка | Команда | Ожидаемый результат |
|---|---|---|
| Scoring: модель, валидация, gRPC, контракт, fixtures | `make scoring-sync && make scoring-test` | Все тесты `ml/scoring/tests` проходят. |
| Assistant: сервер, Help, паритет с Go | `PYTHONPATH=proto/gen/python uv run --project ml/assistant --locked python -m unittest discover -s ml/assistant/tests -v` | Все тесты проходят без Ollama: проверяются шаблоны и разбор ответов модели. |
| Обновление общих fixtures Help (при согласованном изменении рендера) | `PYTHONPATH=proto/gen/python:ml/assistant ml/scoring/.venv/bin/python scripts/generate_help_fixtures.py` | `pkg/assistanthelp/testdata/parity.json` перегенерирован; Go и Python тесты снова зелёные. |

## 4. Smoke-проверки запущенных сервисов

| Скрипт / цель | Предусловие | Что проверяет | Ожидаемый результат |
|---|---|---|---|
| `make scoring-smoke` (`scripts/scoring_smoke.py`) | `make scoring-up` | Общие fixtures `seed/scoring/personas.json` против реального gRPC scoring. | Совпадение баллов, грейдов и причин с fixtures. |
| `make scoring-smoke-go` | `make scoring-up` | Тот же контракт из Go-клиента (`cmd/scoring-smoke`). | Успешный ответ. |
| `make scoring-evaluate` (`scripts/evaluate_scoring.py`) | `make scoring-up` | Согласие с fixtures и латентность RPC. | JSON-отчёт в `/tmp/smart-credit-scoring-evaluation.json`. Это не валидация предсказательной силы. |
| `make bureau-smoke` | `make bureau-up` | gRPC health, профиль, кредитная история, настроенная ошибка, неизвестный ИНН. | `bureau smoke passed: ...`. |
| `make assistant-smoke` (`scripts/assistant_smoke.py`) | `make assistant-up` | Оба RPC `AssistantService` на снимке семи банковских сценариев `ml/assistant/scenarios.json`. | Тексты содержат продукт, грейд, цель, статус и причины; `model_version` равна `OLLAMA_MODEL` или `template`, если Ollama недоступен. |
| `scripts/assistant_help_smoke.py --target 127.0.0.1:50052` | assistant запущен; `PYTHONPATH=proto/gen/python` | `AssistantHelpService/Help` на `ml/assistant/tests/fixtures/help-smoke.json`. | Источник `TEMPLATE` (по умолчанию) или `MODEL` с `--expect-source model`; версия каталога и `context_id` совпадают. |

## 5. Сквозные Docker-сценарии

Каждый скрипт создаёт собственный Compose project со случайными портами и volumes, собирает образы, мигрирует базы и в `finally` удаляет только свой проект. Нужны Docker, Python 3, Go и ресурсы на сборку. С Colima временные файлы должны быть внутри `$HOME`: `TMPDIR=$HOME/.cache/smart-credit-smoke python3 scripts/<script>.py`.

| Скрипт | Что проверяет | Ожидаемый результат |
|---|---|---|
| `make auth-smoke-test` (`scripts/auth_docker_smoke.py` → `TestDockerAuthClientIntegration`) | auth + client-service + gateway: readyz, 401 без токена и с повреждённым токеном, логин, HttpOnly refresh-cookie на `/api/auth` с `SameSite=Strict`, `me`, изоляция организаций по JWT (параметр `org_id` игнорируется), 403 для чужой роли, logout, восстановление gateway после пересоздания контейнеров, неизменность JWKS. | Тест проходит, ключ подписи сохраняется после пересоздания. |
| `scripts/profile_v2_docker_smoke.py` | Профиль v2 через auth, client, Kafka и bank на продукте `SME_PROFILE_V2`: каталог из `bank.events`, черновик с `can_submit`, отправка, `NEEDS_INFO` с требованиями у клиента и банка, загрузка документов по `requirement_id` и периоду, `InfoProvided`. | `E2E v2 catalog → submit → NEEDS_INFO → InfoProvided → <статус> passed`; итог — `MANUAL_REVIEW`, `OFFER`, `COUNTER_OFFER` или `REJECTED`, запрос не открывается повторно. |
| `scripts/assessment_docker_smoke.py` (Ollama намеренно недоступен) | ООО: оценка `READY_FOR_CONFIRMATION` с альтернативами и `baseline_v1`, проекция оценки у клиента, Help с шаблоном, 409 на неизвестный каталог, 403 клиенту на банковскую оценку; recheck: новое поколение, идемпотентный повтор; 422 на ставку вне кандидата; `COUNTER` → принятие → повторная подпись → ровно одна демо-выдача; ИП ПСН: запрос только патента и `MANUAL_REVIEW` после ответа; ликвидируемая компания: `REJECTED` с `COMPANY_LIQUIDATION`; сиды `demo.sql`: маршрутизация обогащения и оценки (`scripts/bank_integration.py`). | `All assessment E2E scenarios passed`. |

Скрипт также дожидается публикации обновлённого каталога со всеми четырьмя целями,
включая `OTHER`, в клиентском сервисе: такой каталог не должен блокировать очередь событий.

Дополнительная проверка восстановления: `python3 scripts/assessment_docker_smoke.py --check-recovery`.
После обычных сценариев скрипт останавливает Kafka и банк только своего временного
Compose-проекта, отправляет новую заявку через клиентский сервис, проверяет сохранение
события в outbox, запускает сервисы и ждёт обработки. Затем перезапускает клиентский
и банковский сервисы: состояние заявки должно сохраниться, повторной выдачи быть не должно.
Этот тест проверяет восстановление после перезапуска, а не доступность при отказе всего сервера.

## 6. Коллекции Postman (`postman/`)

Подготовка — `postman/README.md`: `make demo-up`, импорт пяти коллекций и окружения `Smart-Credit-Local.postman_environment.json`, пароли `client_password`/`analyst_password` из `AUTH_DEV_*` локального `.env`, working directory — папка `postman`. Файл загрузки — `postman/fixtures/bank-statement.txt`: синтетическая выписка в формате `SMART_CREDIT_DEMO_STATEMENT_TXT_V1`, которую принимает парсер `services/bank-service/internal/document/statement` (суммы операций сходятся с оборотами и остатками). client-service сохраняет файл как демовложение и не извлекает из него данные. Запуск — Collection Runner, 1 итерация, задержка 1000 мс, коллекции по одной (общие переменные окружения).

Все коллекции выполняют общие шаги 01–09: логин клиента, профиль ООО «Северная пекарня» (ИНН `7707083893`), каталог с продуктом `WORKING`, черновик, `PATCH` формы (3 000 000 ₽, в `Manual-Reject` — 5 000 000 ₽; 18 мес., `WORKING_CAPITAL`), загрузка выписки (201, `BANK_STATEMENT`), отправка.

| Коллекция | Дальнейшие шаги | Ожидаемый результат |
|---|---|---|
| `Smart-Credit-Cancel` | Отзыв, ожидание, повтор отзыва с тем же ключом, новый отзыв с другим ключом, проверки банка аналитиком. | Клиент `CLOSED`, банк `CANCELLED`, аудит `APPLICATION_CANCELLED`; повтор с тем же ключом — 202, новый ключ после закрытия — 409. |
| `Smart-Credit-Manual-Reject` | Ожидание `MANUAL_REVIEW`, `take`, решение `REJECT`, ожидание у клиента, аудит. | Клиент `REJECTED`, в аудите есть запись с `actor_type = ANALYST`. |
| `Smart-Credit-Offer-Accept` | Ожидание оферты, принятие, ожидание `FORMALIZING`, проверка дела. | Клиент `FORMALIZING`, банк `OFFER_ACCEPTED`. |
| `Smart-Credit-Offer-Decline` | Ожидание оферты, отказ, ожидание `CLOSED`, проверка дела. | Клиент `CLOSED`, банк `CANCELLED`. |
| `Smart-Credit-Sign-Disburse` | Ожидание оферты, принятие, подпись, ожидание выдачи, аудит. | Клиент и банк `DISBURSED`, в аудите `CONTRACT_SIGNED` и `DEMO_DISBURSED`. |

Ограничение текущей версии: коллекции принятия, отказа и подписи были написаны под автоматическую оферту, а конвейер v2 её не выпускает — дело останавливается в `MANUAL_REVIEW`. Шаг «Wait for offer» опрашивает статус до 60 раз; за это время аналитик должен взять дело и подтвердить вариант оценки (кабинет банка или `POST /backoffice/applications/{id}/take` и `/decision`). Иначе шаг завершится сообщением `Expected automatic offer; received ...` или по превышению числа опросов.

## 7. Браузерный сценарий

Стенд с backend поднимает `make demo-up` (веб на `http://localhost:3000`). Каталог берётся из продуктов и правил `services/bank-service/seeds/demo.sql`; демо-дела того же seed client-service пропускает, так как у них нет клиентских заявок. Учётки создаёт `make auth-dev-seed` (входит в `demo-up`), пароли — значения `AUTH_DEV_*` локального `.env` (см. корневой README; все три клиента используют пароль клиента): `client@smart-credit.test` (ООО, ИНН `7707083893`), `ip@smart-credit.test` (ИП на ПСН, ИНН `990000000115`), `closing@smart-credit.test` (ликвидация, ИНН `1111111117`), `analyst@smart-credit.test`. ИНН фиксируется при первом сохранении профиля, поэтому у каждой персоны своя учётка.

| Шаг | Роль | Действие | Ожидаемый результат |
|---|---|---|---|
| 1 | Клиент `client@` | Заполнить профиль ООО с ИНН `7707083893` (на публичном стенде уже заполнен); в «Подобрать кредит» выбрать цель «Товар и текущие расходы» и продукт «Демо: денежный поток» (`SME_CASHFLOW_V2`). | Черновик с планом требований и подсказками. |
| 2 | Клиент | 5 000 000 ₽ / 12 мес.; в блоке «Если всю сумму дать не получится» — «Минимальная сумма» 1 000 000 ₽; «Отправить заявку». | «Отправлена», примерно через 10 с «На рассмотрении»: `VALIDATION → ENRICHMENT → SCORING →` оценка `→ MANUAL_REVIEW`, до трёх расчётных вариантов. Без минимальной суммы банк не уменьшает запрос: «Подходящих условий не нашлось» (`NO_AFFORDABLE_OPTION`). |
| 3 | Аналитик | `/bank`, открыть дело, «Взять в работу»; при необходимости «Пересчитать оценку». | Допуск, источники (`is_mock`), факторы скоркарты v2, график потока, до трёх вариантов, история поколений. |
| 4 | Аналитик | «Предложить этот вариант», заполнить обязательный «Внутренний комментарий» (1–2000 символов) и «Причину для клиента», «Зафиксировать решение». | Оферта или, если сумма меньше запрошенной, контрпредложение с точными условиями варианта. |
| 5 | Клиент | Статус «Есть предложение»: просмотреть варианты и «Помощник», «Принять предложение». | `OFFER_AVAILABLE` → «Оформление» (`FORMALIZING`); Помощник отвечает шаблоном, если Ollama недоступна. |
| 6 | Клиент | Отметить «Понимаю, что это демонстрационная подпись», «Подписать договор». | Через несколько секунд «Деньги выданы» (`DISBURSED`), в аудите банка `CONTRACT_SIGNED` и `DEMO_DISBURSED`. |
| 7 | Клиент `ip@` | Заявка ИП на ПСН на тот же продукт. | Банк запрашивает только патент: «Нужны данные» → загрузка любого файла → «Ответ отправлен» → «На рассмотрении»; дело возвращается аналитику с расчётом. |
| 8 | Клиент `closing@` | Заявка ООО с ИНН `1111111117` (персона `LIQUIDATING` в `seed/personas.yaml`). | «Отказ» (`REJECTED`) без участия аналитика, примерно через 15 с. |
| 9 | Клиент | В черновике «Удалить черновик» и подтвердить. | `DELETE /client/applications/{id}` → 204, черновик исчезает из списка. |

Без backend тот же интерфейс проверяется в демо-режиме `make web-mock` (`http://localhost:5173`, учётки `*@demo.local` с паролем `demo`); правила мока упрощены и не являются источником правил банка, а эндпоинтов оценки и Help в нём нет, поэтому экраны оценки v2 там не показываются. Логика форм и подсказок без браузера — `npm --prefix web test`.
