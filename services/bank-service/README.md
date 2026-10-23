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

Для локальной проверки запустите Postgres, Kafka, миграции, seed продуктов и
новую сборку банка, затем выполните:

```sh
make bank-submitted-smoke
```

Команда напечатает `application_id` и `event_id`. Проверьте дело через Bank API
или SQL в `bank_db`. Для повтора события передайте те же идентификаторы:

```sh
make bank-submitted-smoke APPLICATION_ID=<application_id> EVENT_ID=<event_id>
```

Пока consumer реализует только `ApplicationSubmitted`. Сообщение другого типа
или с нарушенным контрактом остаётся без подтверждения offset и повторяется с
задержкой; текущий последовательный consumer в это время не читает новые
сообщения. Ошибка видна в логах `bank-service`. Следующие типы `client.events`
будут добавляться отдельными ветками.
