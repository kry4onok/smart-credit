# Диаграммы

Корневой README показывает два обзорных SVG: [архитектуру](architecture.svg) и [путь заявки](client-journey.svg). Их делает [D3 7.9](https://d3js.org/) из данных и примитивов в [render.mjs](render.mjs). Цвета взяты из `web/src/styles.css`; SVG открываются в обычном просмотрщике Markdown и не требуют браузерного JavaScript.

Подробные технические схемы остаются исходниками PlantUML: [архитектура](architecture.puml), [заявка → оферта](client-journey.puml), [принятие → демо-выдача](fulfillment.puml). Обзорные SVG и `.puml` описывают одни и те же контуры на разном уровне детализации; при изменении сценария обновляйте оба представления.

## Обновить SVG

Нужны Node.js 18+ и npm. Из каталога `docs/diagrams/`:

```bash
npm ci
npm run render
```

Зависимости зафиксированы в `package-lock.json`. Генератор пишет `architecture.svg` и `client-journey.svg`; полученные файлы коммитьте вместе с `render.mjs`. Для проверки PlantUML отдельно выполните из корня проекта `plantuml -checkonly docs/diagrams/*.puml`. Команда `plantuml -tsvg` поверх обзорных SVG не нужна: она заменит D3-версию техническим рендером.
