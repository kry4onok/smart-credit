# web

Веб-интерфейс двух ролей: кабинет клиента (`/cabinet`) и рабочее место аналитика (`/bank`).
React 19 + Vite + TypeScript, обычный CSS, без UI-фреймворка и state manager. Типы API
генерирует `openapi-typescript` из `../api/*.yaml`, запросы идут через `openapi-fetch`.
В runtime нет Node-сервера: статику отдаёт nginx (`deploy/docker/web.Dockerfile`).

## Запуск

Нужен Node.js 22.12+ (или 20.19+). Из корня репозитория:

```sh
make web-install
make web-mock   # без backend: встроенный мок контрактов, http://localhost:5173
make web-dev    # с backend: /api проксируется на gateway (make gateway-up)
make web-up     # production-сборка в Docker, http://localhost:3000
```

`make web-dev` проксирует `/api` на gateway `http://127.0.0.1:${GATEWAY_PORT}` из
корневого `.env` (другой адрес — `WEB_API_PROXY`, например `http://127.0.0.1:3000`,
чтобы ходить через уже запущенный web-контейнер). В Docker nginx web-контейнера сам
маршрутизирует `/api/auth`, `/api/client`, `/api/backoffice` на `auth`,
`client-service`, `bank-service` внутри compose-сети.

## С настоящим backend

```sh
make infra-up auth-up auth-dev-seed client-up scoring-up bureau-up bank-up
make web-up     # http://localhost:3000
```

Учётки создаёт `make auth-dev-seed`: `client@smart-credit.test`,
`analyst@smart-credit.test`, `admin@smart-credit.test`; пароли — `AUTH_DEV_*` в
локальном `.env`. Каталог продуктов приходит в client-service из `bank.events`.
Для каталога достаточно продуктов и правил из начала
`services/bank-service/seeds/demo.sql`. Демо-дела из того же seed не имеют заявок в
`client_db` и останавливают consumer `bank.events` в client-service, поэтому полный
`make bank-demo-seed` в связке с клиентом не используйте.

Проверено на стенде: вход, профиль, черновик с предзаполнением из калькулятора,
загрузка документа, отправка, автооферта, ручной разбор, «Взять в работу»,
контрпредложение аналитика, принятие оферты и отказ от неё. Подпись и отзыв фронт
отправляет, но bank-service пока не обрабатывает `ContractSigned` и
`ApplicationCancelled`: заявка остаётся в «Подпись отправлена», а consumer
`client.events` останавливается на этом событии.

После принятия client-service больше не возвращает оферту, хотя `POST /sign`
требует её `offer_id`. Web запоминает принятую оферту в `localStorage` и по ней
показывает условия и подпись; в другом браузере подписать нельзя, пока backend не
начнёт отдавать принятую оферту в карточке.

Client-service отдаёт `form` только когда заданы сумма, срок и цель. До этого web
хранит введённые значения черновика в `localStorage`, чтобы они не пропадали после
перезагрузки.

## Демо-режим

`make web-mock` включает `VITE_API_MOCK=true`: запросы обрабатывает
`src/mock/server.ts` в браузере, данные лежат в `localStorage`. Мок воспроизводит
конвейер с задержками (валидация → моки ЕГРЮЛ/БКИ → скоринг → решение) и пять исходов
по ИНН персон из `seed/personas.yaml`. Пароль у всех учёток — `demo`:

| Учётка | Что показывает |
|---|---|
| `bakery@demo.local` | профиль не заполнен → каталог → черновик → автооферта → подпись → выдача |
| `warehouse@demo.local` | заявка уже на ручном разборе (grade C) |
| `service@demo.local` | банк запрашивает данные, после ответа — оферта |
| `workshop@demo.local` | готовый черновик сверх лимита к выручке → аналитик делает контрпредложение |
| `closing@demo.local` | отказ по стоп-фактору: компания ликвидируется |
| `analyst@demo.local` | очередь, «Взять в работу», решение, резюме дела |

Правила мока — упрощённая копия правил bank-service для показа интерфейса, а не их
источник. В production-сборку мок не попадает. Кнопка «Сбросить демо-данные» в
верхней плашке возвращает исходное состояние.

## Устройство

- `src/api/http.ts` — три клиента (`authApi`, `clientApi`, `bankApi`). Access-токен
  только в памяти, refresh — HttpOnly cookie; при `401` один refresh и один повтор.
- `src/api/problem.ts` — `application/problem+json` → понятный текст ошибки.
- `src/components/Pipeline.tsx` — линия конвейера с текущим этапом заявки.
- `src/pages/client`, `src/pages/bank` — экраны ролей. Статусы, `allowed_actions`
  и `next_action` web берёт из ответа сервера и не вычисляет сам; состояние
  обновляется polling-ом, пока заявка в работе у банка.
- Деньги — целые копейки, ставка — basis points; ввод в рублях и процентах
  переводится без float. Все изменяющие POST отправляют `Idempotency-Key`,
  `PATCH`/`take`/`decision` — `If-Match` с версией.

После изменения `api/*.yaml` перегенерируйте типы: `make web-gen`, затем
`npm --prefix web run typecheck`.
