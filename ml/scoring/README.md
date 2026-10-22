# Scoring

Python gRPC-сервис оценки риска бизнеса: `monotone-scorecard-v1`, вход
`borrower-features-v1`. Возвращает индекс 0..1000 (больше — ниже риск), grade
A–E и объяснения сотруднику банка. Это экспертная модель без обучающих данных
и калибровки вероятности дефолта. Кредитное решение принимает bank-service.

## Запуск и контракты

Из корня репозитория:

```sh
make scoring-sync scoring-up
make scoring-smoke scoring-smoke-go
```

Scoring описан в общем `deploy/docker-compose.yml`. Перед первым запуском
создайте `.env` на основе `.env.example`. `make scoring-up` запускает только
scoring, без БД, Kafka и внешнего model API.
Host: `127.0.0.1:50051`; общая Compose-сеть: `scoring:50051`.
Health: стандартный `grpc.health.v1.Health/Check`, имя сервиса
`smartcredit.scoring.ScoringService`; SERVING после загрузки конфига.

- RPC: `smartcredit.scoring.ScoringService/Score`.
- Схемы: `proto/scoring/scoring.proto`, `proto/bureau/bureau.proto`.
- Go: `github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring`.
- Python: `proto.scoring.scoring_pb2`, `proto.scoring.scoring_pb2_grpc`;
  `PYTHONPATH=proto/gen/python` уже задан в командах Makefile.
- Пример работающего Go-клиента: `cmd/scoring-smoke/main.go`.
- Общие исходные профили и ожидаемые ответы: `seed/scoring/personas.json`.

`make gen-proto` генерирует Go/Python для всех пяти proto;
`make gen-proto-check` проверяет отсутствие расхождений. Версии инструментов
закреплены в `scripts/proto-tools.json`; generated код вручную не менять.
В bureau producer и scoring consumer обновлять presence одновременно.

## Вход для backend

Envelope: `application_id` и `product_id` — UUID, версия признаков строго
`borrower-features-v1`. Каталог и допустимость продукта проверяет банк.

| Поле `Features` | Значение и источник |
|---|---|
| `revenue_12m` | Money, выручка из сохранённого финансового снимка заявки |
| `profit_12m` | Money, прибыль из того же снимка; может быть отрицательной |
| `total_debt` | Money, остаток долга из успешного raw CreditHistory текущей попытки |
| `company_age_months` | optional int32, полные месяцы по дате регистрации из успешного CompanyProfile |
| `overdue_count_12m` | optional int32, число эпизодов просрочки из того же CreditHistory |
| `max_overdue_days` | optional int32, максимальный достигнутый DPD в том же окне |

Деньги — целые копейки, `currency=RUB`, абсолютная сумма ≤9007199254740991.
Все активные показатели, кроме прибыли, неотрицательны. Missing Money/scalar
не заменять нулём; известный ноль устанавливать явно, например
`OverdueCount_12M: proto.Int32(0)`. Не собирать вход из HTTP-карточки:
её `COALESCE` уже может подменить отсутствующие сведения БКИ нулями.

Банк один раз фиксирует UTC `as_of_date` на попытку. Окно финансов и просрочек:
`(as_of_date минус 12 календарных месяцев; as_of_date]`; у молодого бизнеса
берутся фактические показатели без пересчёта на год. Долг — остаток на дату.
Эпизод считается, если хотя бы один день попал в окно; DPD — максимум,
достигнутый внутри окна, включая продолжающийся старый эпизод, поэтому возможен
DPD >365. `count=0/days>0` и обратная комбинация противоречивы.

Возраст — разница year/month, минус один, если месячная годовщина ещё не наступила;
день регистрации ограничивается последним днём целевого месяца.
Будущая регистрация — ошибка входа. Fixtures используют свою фиксированную дату
`2026-10-22`, а не системные часы. Сервис дату сам не вычисляет.

`obligations`, ОКВЭД, сумма, срок и цель заявки в формулу v1 не входят;
legacy-поля protobuf сохраняются, но для расчёта необязательны.

## Ответ и банковская обработка

| Результат | Семантика и действие потребителя |
|---|---|
| `ASSESSED` | `score` присутствует, включая 0; grade A≥800, B≥650, C≥500, D≥350, E ниже350 |
| `INSUFFICIENT_DATA` | Нет score; `MISSING_VALUE` и пути полей; запрос сведений клиента либо manual review |
| `UNSUPPORTED_INPUT` | Нет score; `ZERO_REVENUE` или `INCONSISTENT_OVERDUE`; manual review |
| `INVALID_ARGUMENT` | Неверные UUID/валюта/диапазон/отрицательное значение; без повтора, технический аудит |
| `FAILED_PRECONDITION` | Неизвестная версия признаков; без повтора, исправить совместимость |
| Остальные gRPC-ошибки | Технический outcome и manual review, без запасного score |

При отсутствии оценки grade=UNSPECIFIED, причины и вклады пусты. Версии,
baseline и checksum остаются. Неуказанный `assessment_status` не считается
результатом v1. Порядок проверки: структура/версия → пропуски → применимость.
Отсутствующий долг вместе с нулевой выручкой даёт INSUFFICIENT_DATA.

Успешный ответ содержит четыре signed-вклада: `business_age`, `profit_margin`,
`bureau_debt_load`, `payment_discipline`; `baseline_score + sum(points) == score`.
До трёх причин отсортированы по `(-contribution, code)`; contribution абсолютный,
знак задан direction. `NO_OVERDUE` применяется только при известных `0/0`.
`config_checksum` — SHA-256 точных байтов загруженного артефакта. Формула и
проверенные параметры находятся в `models/scorecard-v1.json` и `model.py`;
повторно реализовывать их в Go не нужно.

Общий бюджет вызова — 2 с, не более одного повтора UNAVAILABLE внутри того же
бюджета. Вызов выполняется вне долгой SQL-транзакции. Банк сохраняет вход,
даты, metadata и outcome с `input_revision` и `attempt_id`; поздний ответ
старой попытки не заменяет текущий. При новом входе/проверке прежний score
снимается с позиции текущего. Case.version не заменяет input_revision.
Только ASSESSED записывается в существующий `score_results`, где score NOT NULL;
остальные исходы сохраняются отдельно как outcome попытки.

Bank OpenAPI поддерживает представление доменного ответа без score. Хранение
outcome и чтение актуальной попытки должны предшествовать его отображению.
Grade не обходит стоп-факторы, документы и продуктовые правила. Внутренние
оценки/причины не передаются клиенту в `bank.events`. Логи — идентификаторы,
версии, длительность и outcome, без финансового payload и документов.

## Проверка и зависимости

```sh
make scoring-test scoring-lint gen-proto-check
go test ./...
make scoring-evaluate
make scoring-down
```

Измерения сохраняются в `/tmp/smart-credit-scoring-evaluation.json`;
synthetic fixtures проверяют контракт, а не качество прогноза дефолта.
Без Docker: `make scoring-run`; адрес задаёт `SCORING_ADDR` (по умолчанию
loopback), артефакт — `SCORING_MODEL_PATH` (по умолчанию включённый в пакет).
CPU, лимиты Compose — 1 CPU/256 MiB; GPU не нужен.

Python 3.12.12 и зависимости закреплены в `pyproject.toml`/`uv.lock`.
Runtime: grpcio и grpcio-health-checking (Apache-2.0), protobuf (BSD-3-Clause).
Go: grpc-go (Apache-2.0), protobuf-go (BSD-3-Clause). Конфиг и personas —
собственные артефакты, без внешнего датасета и pretrained weights; отдельная
лицензия модели не объявлена. Upstream-зависимости сохраняют свои лицензии:
[gRPC](https://github.com/grpc/grpc),
[protobuf](https://github.com/protocolbuffers/protobuf),
[grpc-go](https://github.com/grpc/grpc-go),
[protobuf-go](https://github.com/protocolbuffers/protobuf-go).
