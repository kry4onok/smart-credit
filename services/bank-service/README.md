# Приём `ApplicationSubmitted`

`bank-service` читает protobuf `Event` из `client.events` (ключ Kafka —
`aggregate_id`). Адреса брокеров задаёт `BANK_KAFKA_BROKERS` через запятую;
локальное значение по умолчанию — `127.0.0.1:9092`, в Docker Compose —
`kafka:19092`. Группа задаётся `BANK_KAFKA_CONSUMER_GROUP` (по умолчанию
`bank-service`). При временной недоступности Kafka процесс продолжает работать,
consumer переподключается.

Для нового события в одной транзакции создаются `cases`, `audit_log` и
`processed_events`. Известная версия продукта даёт статус `VALIDATION` и
`next_run_at`; отсутствующая версия остаётся доступной аналитикам в
`MANUAL_REVIEW` с причиной `UNKNOWN_PRODUCT`. Повтор того же `event_id`
не создаёт второе дело. Offset подтверждается только после commit.

Worker `VALIDATION` проверяет сохранённую заявку по версии выбранного продукта:
наличие суммы и срока, цель, финансовые поля компании и метаданные обязательных
документов. Полная заявка переходит в `ENRICHMENT`. При нехватке данных создаётся
открытый `info_requests`, дело переходит в `NEEDS_INFO`, а в outbox записывается
`ApplicationPublicStatePublished` со списком нужных полей и документов. Переход,
аудит, запрос и событие записываются одной транзакцией. Публикацию outbox в Kafka
выполнит отдельный relay; пока публичное событие находится только в БД.

Для локальной проверки запустите Postgres, Kafka, миграции, seed продуктов и
новую сборку банка, затем выполните:

```sh
make bank-submitted-smoke
```

Команда напечатает `application_id` и `event_id`. Её заявка содержит все поля и
документ, поэтому worker переведёт дело дальше в `ENRICHMENT`. В Swagger Bank API
откройте `GET /api/backoffice/applications/{application_id}` с напечатанным UUID: фактический
статус может быть уже `SCORING` или `DECISION`, если запущены следующие worker'ы.
Для повтора события передайте те же идентификаторы:

```sh
make bank-submitted-smoke APPLICATION_ID=<application_id> EVENT_ID=<event_id>
```

Для проверки `NEEDS_INFO` отправьте отдельную неполную заявку:

```sh
make bank-submitted-smoke MISSING_DATA=1
```

Отправитель уберёт выручку и документы. В карточке дела должны появиться статус
`NEEDS_INFO` и активный запрос с `COMPANY_REVENUE_12M` и `BANK_STATEMENT`.

Пока consumer реализует только `ApplicationSubmitted`. Сообщение другого типа
или с нарушенным контрактом остаётся без подтверждения offset и повторяется с
задержкой; текущий последовательный consumer в это время не читает новые
сообщения. Ошибка видна в логах `bank-service`. Следующие типы `client.events`
будут добавляться отдельными ветками.
