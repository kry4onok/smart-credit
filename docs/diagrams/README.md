# Диаграммы

Исходники находятся рядом с опубликованными SVG: `architecture.puml`, `client-journey.puml`, `branches.puml`. Корневой README встраивает SVG, поэтому схемы видны в обычном просмотрщике Markdown без сервера рендера; `.puml` остаются редактируемыми.

Архитектура использует встроенную библиотеку **C4-PlantUML** (`!include <C4/C4_Container>`); sequence-диаграммы используют штатный синтаксис PlantUML и общую палитру. Внешних URL в исходниках нет. SVG проверены на PlantUML `1.2026.8` с Graphviz. Чтобы обновить их после изменения `.puml`, установите [официальный PlantUML](https://github.com/plantuml/plantuml/releases) и выполните из корня проекта:

```bash
plantuml -tsvg docs/diagrams/*.puml
plantuml -checkonly docs/diagrams/*.puml
```

Коммитьте изменённый `.puml` вместе с соответствующим `.svg`.
