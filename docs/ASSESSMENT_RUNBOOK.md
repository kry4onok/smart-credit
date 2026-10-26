# Запуск оценки v2 и Help

## Окружение

Используются существующие Compose, Makefile, PostgreSQL, Kafka, auth/client/bank, Python scoring, bureau-mocks и assistant. Frontend в этот объём не входит. Не заменяйте существующий `.env`: для нового окружения скопируйте `.env.example` и задайте локальные значения.

```sh
cp .env.example .env  # только если файла ещё нет
make demo-up
```

Swagger: `http://localhost:8000/docs/`, выбрать **Assessment and Help API**. Описание новых ручек: `api/assessment.yaml`, существующего процесса: `api/client.yaml`, `api/bank.yaml`, `api/auth.yaml`. Демо-продукт нового процесса `10000000-0000-4000-8000-000000000022` / `SME_CASHFLOW_V2` / `sme-profile-v2`. Прежний `sme-profile-v1` сохраняет свой перечень обязательных полей.

Миграции: bank `00007_assessment.sql`, client `00003_assessment.sql`, применяются штатными `bank-db-up`/`client-db-up`. Они добавляют поля и таблицы, не пересчитывают старые оценки. Down отказывается удалять сохранённые новые данные; откат заполненного окружения требует заранее проверенного восстановления резервной копии. Не используйте `down --volumes` для рабочего проекта.

Внешние сведения в MVP синтетические; значения не являются полученными из реальных реестров. Предварительный маршрут использует данные источников перед запросом документов. Загруженный файл считается PROVIDED, но не доказанно VERIFIED: извлечения произвольной отчётности и OCR нет.

## Qwen и автономная работа

Банковский `BANK_ASSISTANT_GRPC_ADDR` и клиентский `CLIENT_ASSISTANT_TARGET` указывают на `assistant:50052` в Compose. У assistant используется существующий Ollama HTTP API и настройки `OLLAMA_URL`, `OLLAMA_MODEL` из `.env.example`. Модель должна быть заранее доступна на выбранном Ollama сервере. Сервис не обучает и не скачивает модель автоматически.

Новый RPC `smartcredit.assistant.v2.AssistantHelpService/Help` сосуществует с прежним сервисом. Контракт: `proto/assistant/help.proto`. Каталог: `pkg/assistanthelp/catalog.json`, версия `smart-credit-help-v1`. Контекст формируется backend после авторизации; Qwen выбирает только разрешённые statement/variant/action IDs. Go заново рендерит текст из доверенного контекста и проверяет актуальность после RPC. Действия являются навигацией и не исполняют команды банка.

При недоступной модели, таймауте, очереди или неверном ответе используется детерминированный шаблон известного каталога. Недоступность всего assistant также покрыта Go fallback. Неизвестный каталог возвращает явную несовместимость, а устаревший контекст — 409 с необходимостью заново получить контекст. Для проверки автономного режима можно направить `OLLAMA_URL=http://127.0.0.1:1`; текущий E2E сценарий делает это только в своём временном окружении.

## Воспроизводимая проверка

```sh
make lint
make gen-proto-check
go test -race ./...
PYTHONPATH=proto/gen/python:ml/scoring ml/scoring/.venv/bin/python -m pytest ml/scoring/tests -q
PYTHONPATH=proto/gen/python:ml/assistant ml/scoring/.venv/bin/python -m unittest discover -s ml/assistant/tests -q
python3 scripts/assessment_docker_smoke.py
```

Для SQL-тестов задать `BANK_DATABASE_URL` (все bank миграции применены), `CLIENT_TEST_DATABASE_URL`, `AUTH_TEST_DATABASE_URL` на выделенные тестовые базы. Без этих переменных соответствующие интеграционные тесты пропускаются. Client/auth тесты создают отдельные схемы; bank тесты используют свои UUID/транзакции. Не направляйте тесты на продуктивную БД.

`assessment_docker_smoke.py` создаёт уникальный Compose project, случайные локальные порты и собственные volumes; собирает образы, мигрирует базы и удаляет только свой проект в `finally`. Он проверяет ООО с альтернативами, недоступный Qwen, несовместимый каталог, повторную оценку и replay, запрещённую ставку, подтверждение → принятие → повторное подписание → ровно одну демонстрационную выдачу; ИП ПСН с запросом патента; отказ ликвидируемой компании. Требуются Docker Desktop, Python, Go и ресурсы для сборки. С Colima временные файлы должны лежать внутри `$HOME` (VM видит только его): `TMPDIR=$HOME/.cache/smart-credit-smoke python3 scripts/assessment_docker_smoke.py`; то же для `profile_v2_docker_smoke.py`.

30 общих полных fixtures лежат в `pkg/assistanthelp/testdata/parity.json`: их читают Python и Go тесты. При согласованном изменении рендера обновление выполняется командой:

```sh
PYTHONPATH=proto/gen/python:ml/assistant ml/scoring/.venv/bin/python scripts/generate_help_fixtures.py
```

Сравниваются блоки, текст, отсылки к evidence, действия, статусы и версии. Это не оценка качества ответов реальной Qwen. Для такого измерения отдельно нужен доступный Ollama с выбранной моделью и прогон тех же контекстов с фиксацией латентности/валидности выбора ID.
