# Диаграммы Smart Credit

В корневом README используется [карта архитектуры](architecture.svg). Вертикальные схемы [клиента](client-flow.svg), [банка](bank-flow.svg) и [авторизации](auth-flow.svg) встроены в README соответствующих сервисов. В каждой показаны основной путь, статусы и боковые исходы.

Все схемы имеют редактируемые исходники `.puml` в этом каталоге: [архитектура](architecture.puml), [клиент](client-flow.puml), [банк](bank-flow.puml), [auth](auth-flow.puml). Для подробной технической последовательности сохранены [заявка → оферта](client-journey.puml) и [оферта → демо-выдача](fulfillment.puml). Публичные SVG построены на [D3](https://d3js.org/) скриптом [render.mjs](render.mjs); `.puml` фиксируют ту же логику в стандартном формате PlantUML. При изменении сценария обновляйте оба исходника.

## Обновление SVG

Нужны Node.js 18+ и npm **только разработчику документации**. Из `docs/diagrams/`:

```bash
npm ci
npm run render
```

Зависимости зафиксированы в `package-lock.json`. Генератор пишет `architecture.svg`, `client-flow.svg`, `bank-flow.svg` и `auth-flow.svg`. Для проверки PlantUML из корня репозитория: `plantuml -checkonly docs/diagrams/*.puml`. Генерация SVG не входит в `make demo-up` — готовые файлы уже сохранены в репозитории.
