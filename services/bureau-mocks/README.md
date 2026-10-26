# bureau-mocks

Локальный Go gRPC-сервис для воспроизводимых проверок без внешнего БКИ или реестра. При запуске читает вымышленные персоны из [`seed/personas.yaml`](../../seed/personas.yaml); собственной БД нет. `bank-service` вызывает его на этапе `ENRICHMENT`.

## Контракт

[`proto/bureau/bureau.proto`](../../proto/bureau/bureau.proto) описывает `GetCompanyProfile` (статус и дата регистрации компании), `GetCreditHistory` (долг и просрочки) и `GetAssessmentSources` для оценки v2. Ответы одной персоны детерминированы; seed может задавать задержку или ошибку источника. Неизвестный валидный ИНН возвращает `NOT_FOUND`, невалидный — `INVALID_ARGUMENT`. Сервис предоставляет стандартный gRPC health; reflection включается только настройкой `BUREAU_ENABLE_REFLECTION=true`.

## Запуск и проверка

Из корня репозитория после создания локального `.env`:

```bash
make bureau-up
make bureau-smoke
make bureau-logs
```

В Compose адрес `bureau-mocks:50053`, с компьютера по умолчанию `127.0.0.1:50053` (`BUREAU_PORT` в `.env`). `make bureau-down` удаляет контейнер. Внутренний путь seed задаёт `BUREAU_PERSONAS_PATH`; обычный запуск использует `seed/personas.yaml`. Unit-тесты: `go test ./services/bureau-mocks/...`; связь профильных сидов и моков: `make profile-seed-check`.
