# ML assistant

`SummarizeCase` составляет короткое резюме для аналитика. `ExplainDecision` формирует уведомление клиенту по уже принятому банком решению. Ассистент не принимает решение и не меняет скоринг.

## Запуск через Docker Compose

Ollama запускается на компьютере отдельно от Docker. Установите [Ollama](https://ollama.com/download), запустите приложение или `ollama serve` и загрузите модель:

```bash
ollama pull qwen2.5:3b
ollama list
```

Из корня репозитория запустите assistant и проверьте оба метода:

```bash
cp .env.example .env  # только при первом запуске
make assistant-up
make assistant-smoke
```

Сервис слушает `assistant:50052` в сети Compose и `127.0.0.1:50052` на компьютере. Контейнер обращается к Ollama по `http://host.docker.internal:11434`. Для других адресов и модели задайте `OLLAMA_URL`, `OLLAMA_MODEL` и `ASSISTANT_PORT` в `.env`. На Docker Desktop для Mac `host.docker.internal` указывает на компьютер; при запуске в другой среде проверьте доступ к Ollama из контейнера.

`make assistant-smoke` печатает текст и `model_version` обоих методов и проверяет, что ответила выбранная модель. Значение `template` означает fallback внутри assistant. Для остановки используйте `make assistant-down`, для просмотра логов — `make assistant-logs`.

## Локальный запуск без Docker

Нужны Python 3.12 и запущенная [Ollama](https://ollama.com/download) с моделью `qwen2.5:3b`. Ollama работает вне Docker. Из корня репозитория создайте окружение и установите зависимости из `pyproject.toml`:

```bash
python3.12 -m venv ml/assistant/.venv
ml/assistant/.venv/bin/python -m pip install 'grpcio==1.78.0' 'protobuf==6.33.5'
ollama pull qwen2.5:3b
PYTHONPATH=proto/gen/python OLLAMA_MODEL=qwen2.5:3b ml/assistant/.venv/bin/python ml/assistant/server.py
```

Если окружение уже создано, первую команду можно пропустить. Адрес gRPC по умолчанию — `127.0.0.1:50052`; адрес Ollama — `http://127.0.0.1:11434`. Их можно изменить через `ASSISTANT_ADDR` и `OLLAMA_URL`.

Для ручной проверки обоих методов можно использовать `grpcui -plaintext -proto proto/assistant/assistant.proto 127.0.0.1:50052`.

## Контракт с bank-service

`bank-service` — единственный штатный клиент gRPC-сервиса. Его target в Compose: `assistant:50052`. Compose задаёт `BANK_ASSISTANT_GRPC_ADDR`, но текущий Go-код ещё не читает эту переменную. Контракт определён в `proto/assistant/assistant.proto`.

| Метод | Что передаёт bank-service | Что использует assistant |
|---|---|---|
| `SummarizeCase` | `application_id`, `product_name`, `purpose`, `grade`, `reason_codes`, `triggered_rule_codes` | Название продукта, цель, grade и переданные коды дословно. `application_id` в текст не попадает. Коды не расшифровываются моделью. |
| `ExplainDecision` | `application_id`, уже принятое `outcome`, `product_name`, `public_reason_codes`, безопасный `analyst_public_reason` | Исход решения, продукт и публичную причину. `application_id` и `public_reason_codes` сейчас в текст не попадают; непрозрачные коды не превращаются в выдуманные объяснения. |

Банк выбирает и проверяет публичную причину до вызова `ExplainDecision`. Если клиенту нужно сообщить конкретную причину, её следует передать готовой фразой в `analyst_public_reason`. Внутренний `comment` аналитика, сырые персональные данные и внутренние причины скоринга передавать в клиентское объяснение нельзя.

Backend выставляет `source=assistant`, когда получен текст с версией модели; `model_version=template` отображает как `source=template`. При ошибке gRPC или истечении дедлайна backend формирует собственный шаблон и также ставит `source=template`. Для HTTP-резюме рекомендуется дедлайн gRPC не более 20 секунд при текущем `BANK_HTTP_WRITE_TIMEOUT=30s`. Вызов assistant не должен менять score, решение или оферту. Подключение HTTP-метода резюме, вызова объяснения и публикации текста — задача владельца bank-service; их работа через UI и событие пока не проверена.

## Ограничение фактов и fallback

Ollama отвечает в JSON-формате с температурой `0`. Для резюме аналитика сервис проверяет, что ответ содержит переданные значения и не добавляет известных опасных утверждений. Если проверка не проходит или Ollama недоступна, сервис возвращает короткое резюме из исходных полей с `model_version=template`.

Для клиентского текста Ollama выбирает только тон (`neutral` или `empathetic`). Статус и публичная причина подставляются программой из запроса; внутренние коды без расшифровки не публикуются. При ошибке Ollama используется нейтральный шаблон с `model_version=template`. Backend должен учитывать этот маркер при выставлении `source=template`; если сам gRPC-сервис недоступен или истёк дедлайн, fallback остаётся на стороне bank-service.

Тесты без Ollama:

```bash
PYTHONDONTWRITEBYTECODE=1 ml/assistant/.venv/bin/python -m unittest discover -s ml/assistant/tests -v
```

В контейнерном smoke используются синтетические данные. Проверка fallback без остановки Ollama:

```bash
OLLAMA_URL=http://host.docker.internal:1 docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --no-build --wait assistant
docker compose --env-file .env -f deploy/docker-compose.yml exec -T assistant .venv/bin/python /app/scripts/assistant_smoke.py --expect-version template
make assistant-up  # восстановить обычный адрес Ollama
```

Если выключен сам контейнер assistant, шаблон должен сформировать bank-service после подключения его gRPC-клиента.

CI `.github/workflows/assistant.yml` запускает unit-тесты и проверяет оба метода контейнера при недоступной Ollama. Для проверки реальных ответов модели запустите `make assistant-smoke` на компьютере с установленной Ollama.
