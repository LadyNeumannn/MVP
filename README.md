## Авито.Кухня — MVP

Небольшой MVP сервиса доставки еды из заведений. В проекте два независимых Go-сервиса:

* `services/kuhnya` — основная площадка с Client API и Partner API;
* `services/restaurant-example` — пример сервиса заведения, который синхронизирует меню, получает новые заказы и меняет их статусы.

Для хранения данных используется PostgreSQL.

Клиентскую часть по заданию реализовывать не требовалось. Я оставила небольшой demo-client для ручной проверки сценариев.

## Быстрый старт

```bash
docker compose up --build
```

Поднимутся: Postgres (с применёнными миграциями), `kuhnya` на
`localhost:8080`, `restaurant-example` на `localhost:8090` (сам
регистрируется в `kuhnya` при старте и синхронизирует меню).

Если уже есть volume `pgdata` от предыдущего запуска (до
миграции `0003_delivery_info`) миграции из `db/init/` заново не
выполнятся (Postgres применяет `docker-entrypoint-initdb.d` только при
первой инициализации тома). Делаем `docker compose down -v` перед
`up --build`, чтобы начать с чистой БД.

Проверить сценарий целиком:

```bash
# 1. Список заведений (можно с поиском по названию)
curl localhost:8080/api/v1/restaurants
curl "localhost:8080/api/v1/restaurants?q=пельм"

# 2. Меню (restaurant_id из ответа выше;
#    в дефолтной конфигурации это 6f2f8c1a-0000-4b1a-9c3a-000000000000).
#    Тоже с поиском: ?q=пельмени
curl localhost:8080/api/v1/restaurants/6f2f8c1a-0000-4b1a-9c3a-000000000000/menu

# 3. Оформить заказ (menu_item_id из меню выше). customer_phone и
#    delivery_address обязательны это сервис доставки.
curl -X POST localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-key-1' \
  -d '{
    "restaurant_id": "6f2f8c1a-0000-4b1a-9c3a-000000000000",
    "customer_name": "Ариана",
    "customer_phone": "+79990000000",
    "delivery_address": "г. Нижний Новгород, ул. Тестовая, 1",
    "items": [{"menu_item_id": "6f2f8c1a-0001-4b1a-9c3a-000000000001", "quantity": 2}]
  }'

# 4. Следить за статусом (id заказа из ответа выше). restaurant-example
#    сам решает: ~15% заказов отменяет (status = cancelled, имитация
#    "не хватило ингредиентов"), остальные продвигает created ->
#    confirmed -> preparing -> ready -> delivered с паузами
curl localhost:8080/api/v1/orders/<order_id>

# 5. Статистика заведения (после того как хотя бы один заказ дойдёт
#    до delivered). Требует api_key заведения => посмотри его в логах:
#    docker compose logs restaurant-example | grep api_key
curl localhost:8080/partner/v1/restaurants/6f2f8c1a-0000-4b1a-9c3a-000000000000/stats \
  -H 'Authorization: Bearer <api_key из логов restaurant-example>'
```

## Структура репозитория

```
docs/
  cjm/            — CJM пользователя и заведения (PlantUML)
  c4/             — архитектура: C4 Level 2 (Container) и Level 3 (Component)
  db-schema.md    — описание схемы БД
migrations/       — миграции (up/down)
db/init/          — копии *up*-миграций для автозапуска Postgres в Compose
services/
  kuhnya/
    api/              — OpenAPI-спеки (client-api.yaml, partner-api.yaml),
                        встроены в бинарник через go:embed и раздаются
                        сервисом же на /docs (Swagger UI) и /docs/*.yaml
    ...               — основной сервис (Go)
  restaurant-example/ — пример сервиса заведения (Go)
demo-client/      — необязательный HTML/JS клиент для ручной проверки
.github/workflows/ci.yml — CI/CD: линт + тесты + сборка и публикация
                           образов в GHCR
AI_PROMPT.md      — план/промпты, по которым писался код (бонус по заданию)
docker-compose.yml
.golangci.yml
```

## Архитектура

C4-диаграммы — `docs/c4/c4-container.puml` (Level 2) и
`docs/c4/c4-component.puml` (Level 3, детализация `kuhnya`). Открыть
можно через любой PlantUML-рендерер.

`kuhnya` — основной сервис системы. Он принимает запросы от web-клиента через Client API и от сервисов заведений через Partner API.

`restaurant-example` — отдельный сервис, который взаимодействует с `kuhnya` по HTTP. Общих Go-пакетов между сервисами нет: интеграция построена через API-контракт и webhook, как с внешним партнёром.

Внутри `kuhnya` используется простое разделение на слои:

`handler` → `service` → `storage` → `postgres`

Бизнес-логика находится в `service` и не зависит от SQL. `storage` — интерфейс, поэтому в тестах его можно заменить на in-memory реализацию.

## API

- Клиентский: [`services/kuhnya/api/client-api.yaml`](services/kuhnya/api/client-api.yaml)
  — `GET /restaurants`, `GET /restaurants/{id}/menu`, `POST /orders`,
  `GET /orders/{id}`.
- Партнёрский: [`services/kuhnya/api/partner-api.yaml`](services/kuhnya/api/partner-api.yaml)
  — регистрация заведения, CRUD меню, список заказов, смена статуса,
  статистика (`GET /restaurants/{id}/stats`).
- **Swagger UI** — `GET /docs` (сам `kuhnya`, когда сервис запущен),
  раздаёт обе спеки выше через встроенный в бинарник `go:embed`
  (`services/kuhnya/api/embed.go`)

## Авторизация Partner API

Регистрация заведения (`POST /partner/v1/restaurants`) защищена bootstrap-секретом платформы:

`Authorization: Bearer <PARTNER_ONBOARDING_SECRET>`

После регистрации заведению выдаётся собственный `api_key`. Все остальные методы Partner API требуют этот ключ.

В MVP ключ хранится в БД в открытом виде. В production я бы хранила только хэш ключа и добавила отдельный механизм ротации.

Client API без авторизации.

## Схема БД

Подробно можно посмотреть вот тут => [`docs/db-schema.md`](docs/db-schema.md). Коротко: 4 таблицы
(`restaurants`, `menu_items`, `orders`, `order_items`) + `idempotency_keys`
для защиты от дублей заказа. Заказ хранит снапшот цены/названия позиции
на момент покупки, а не ссылку на текущее состояние меню.

## Реализованные бизнес-сценарии и edge cases

- Заказ невозможен, если заведение неактивно, если позиция не
  принадлежит этому заведению, или если позиция недоступна
  (`available = false`) — всё проверяется атомарно в одной транзакции
  с блокировкой строк, чтобы исключить гонку между проверкой и записью.
- Заказ требует контактный телефон и адрес доставки (`customer_phone`,
  `delivery_address`) — без них 422: это сервис *доставки*, без адреса
  заказ некуда везти.
- Поиск: `GET /restaurants?q=` и `GET /restaurants/{id}/menu?q=` —
  подстроковый регистронезависимый фильтр по названию. 
- Повторный `POST /orders` с тем же `Idempotency-Key` возвращает уже
  созданный заказ вместо дубликата (реалистичный кейс: ретрай на
  нестабильной сети, двойной клик).
- Смена статуса заказа проверяет допустимость перехода (нельзя,
  например, из `delivered` обратно в `created`). Пример-заведение
  реально пользуется этим: ~15% заказов случайно отменяет (имитация
  «не хватило ингредиентов»), остальные доводит до `delivered` — так
  ветка `cancelled` из CJM отрабатывает не только на бумаге.
- Повторная синхронизация меню сервисом заведения (например, после
  перезапуска) идемпотентна — позиции обновляются, а не дублируются.
- Если webhook о новом заказе не доставился (заведение недоступно) —
  заказ всё равно создан и виден заведению через
  `GET /partner/v1/restaurants/{id}/orders?status=created`; отправка
  клиенту не блокируется и не падает из-за недоступности заведения.
- Статистика заведения (`GET /partner/v1/restaurants/{id}/stats`)
  считает выручку только по `delivered`-заказам одним SQL-запросом
  (`GROUP BY status`) — остальные статусы ещё могут быть отменены и не
  являются совершённой продажей.

## Допустимые упрощения MVP (и как это масштабировать дальше)
Что я бы изменила для production

В текущем варианте некоторые вещи намеренно упрощены, чтобы не раздувать MVP:

* Client API не требует авторизации;
* Partner API использует `api_key`, который сейчас хранится без хэширования;
* списки пока без пагинации;
* webhook доставляется отдельным воркером внутри `kuhnya`;
* у webhook нет dead-letter после определённого числа попыток;
* нет rate limiting, метрик и трассировки;
* поиск сделан через `ILIKE '%...%'`;
* `idempotency_keys` пока не очищаются по TTL;
* миграции запускаются через `docker-entrypoint-initdb.d`.

При росте проекта я бы отдельно занялась пагинацией, очередью для webhook, наблюдаемостью, rate limiting и поиском через `pg_trgm` или отдельный поисковый сервис.

Для доставки webhook используется transactional outbox.

Событие о новом заказе записывается в `webhook_outbox` в той же транзакции, что и сам заказ. Отдельный воркер забирает события и отправляет их с повторными попытками и exponential backoff.

Поэтому падение `kuhnya` после создания заказа не приводит к потере уведомления.

Пока это один воркер внутри процесса. Отдельной очереди и dead-letter queue нет — для MVP этого достаточно.

## CI/CD

`.github/workflows/ci.yml` — три job'а на каждый push/PR:

1. **test** — `go vet` + `go test ./...` для обоих сервисов, плюс для
   `kuhnya` отдельным шагом: поднимается job-level Postgres service,
   накатываются `migrations/*.up.sql` через `psql`, и запускаются
   Postgres-интеграционные тесты (`go test -tags=integration`).
2. **lint** — `golangci-lint` для обоих сервисов.
3. **build-and-push** — только при пуше в `main`, после того как test
   и lint прошли: собирает Docker-образы обоих сервисов и публикует их
   в GitHub Container Registry (`ghcr.io/<repo>/kuhnya` и
   `.../restaurant-example`), теги `latest` и `<git-sha>`. 

## Тесты

```bash
# Юнит + HTTP-уровня (без БД, быстро)
cd services/kuhnya && go test ./...

# Интеграционные (реальный Postgres, отдельный build-тег)
export TEST_DATABASE_URL="postgres://kuhnya:kuhnya@localhost:5432/kuhnya_test?sslmode=disable"
go test -tags=integration ./... -run Postgres
```
Покрыты четыре уровня:

* `domain` — допустимые переходы статусов;
* `service` — бизнес-правила, idempotency, `api_key`, outbox;
* `handler` — HTTP, роутинг и авторизация;
* `storage` — PostgreSQL-тесты для транзакций, idempotency и статистики.

Первые три уровня не требуют реальной БД. PostgreSQL-тесты запускаются отдельно с build-тегом `integration`.

Отдельных нагрузочных тестов пока нет.


## Линтер

`.golangci.yml` в корне общий для обоих Go-модулей (`golangci-lint`
ищет конфиг вверх по дереву каталогов). Запуск из каждого сервиса:

```bash
cd services/kuhnya && golangci-lint run ./...
cd services/restaurant-example && golangci-lint run ./...
```
