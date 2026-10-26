# Диаграммы Smart Credit

[Карта контейнеров и данных](architecture.svg) и [прямые вызовы с протоколами](integrations.svg) встроены в корневой README. Вертикальные карты [клиента](client-flow.svg), [банковского дела](bank-flow.svg) и [авторизации](auth-flow.svg) встроены в README соответствующих сервисов. Технические последовательности: [заявка → оферта](client-journey.svg) и [принятие → демо-выдача](fulfillment.svg). Схемы ключевых таблиц трёх баз: [auth_db](auth-db.svg), [client_db](client-db.svg), [bank_db](bank-db.svg); их полный состав описан в миграциях сервисов.

Все SVG в этом каталоге собираются **непосредственно из одноимённых `.puml`**. Карты контейнеров используют встроенную в PlantUML библиотеку [C4-PlantUML](https://github.com/plantuml-stdlib/C4-PlantUML); вертикальные карты показывают основной путь, статусы и развилки. Подача вдохновлена [диаграммами avito-kitchen](https://github.com/exf1kzz/avito-kitchen), содержание сверено с кодом Smart Credit.

## Обновление SVG

Нужен PlantUML с C4-stdlib (проверено на версии `1.2026.8`) и Graphviz. Из корня репозитория:

```bash
plantuml -checkonly docs/diagrams/*.puml
plantuml -tsvg docs/diagrams/*.puml
```

Если PlantUML установлен как JAR, замените `plantuml` на `java -jar /путь/к/plantuml.jar`. Сборка диаграмм нужна только при изменении документации; `make demo-up` использует готовые SVG из репозитория. После редактирования проверьте исходник, SVG и ссылки в README соответствующего сервиса.
