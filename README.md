# Payment Processing

Это простой backend для внутренних переводов между счетами.

Что тут есть:
- регистрация и логин
- JWT auth
- refresh / logout / logout-all
- получение аккаунта и истории операций
- перевод денег между счетами
- rate limiting через Redis
- PostgreSQL для балансов и идемпотентности
- миграции и seed для development

## Быстрый запуск

Нужны Docker и Docker Compose:

```bash
docker compose up --build
```

После запуска сервис будет доступен по адресу:

```text
http://localhost:8080
```

Проверка готовности:

```bash
make api-health-test
```

## Demo-пользователи

В `development` режиме уже есть два пользователя:

- sender@example.test / DemoPass123!
- receiver@example.test / DemoPass123!

Их счета и стартовые балансы уже созданы через seed.

## Примеры

Готовые smoke-проверки уже есть в make:

```bash
make api-auth-test
make api-accounts-test
```

Если хочешь вручную сделать запрос на перевод, можно так:

```bash
curl -X POST http://localhost:8080/transactions \
  -H "Authorization: Bearer <access_token>" \
  -H "Idempotency-Key: demo-transfer-1" \
  -H "Content-Type: application/json" \
  -d '{"receiver_id":"22222222-2222-2222-2222-222222222222","amount":"125.50"}'
```

Важно:
- одинаковый `Idempotency-Key` + тот же payload = тот же результат
- тот же `Idempotency-Key` + другой payload = `409 Conflict`

## Основные роуты

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `POST /auth/logout-all`
- `GET /accounts/{id}`
- `GET /accounts/{id}/transactions`
- `POST /transactions`
- `GET /transactions/{id}`
- `GET /health/live`
- `GET /health/ready`

## Что стоит знать

- PostgreSQL — источник правды для балансов и операций
- Redis — для rate limiting
- переводы делаются внутри одной SQL-транзакции
- суммы проверяются до записи, чтобы не было некорректных значений

## Проверки

```bash
go test ./internal/... ./cmd/server/...
```

E2E тесты требуют Docker:

```bash
go test ./e2e/...
```

## Структура

- `cmd/server` — запуск API
- `cmd/migrate` — миграции
- `cmd/seed` — demo seed
- `internal/delivery/http` — HTTP слой
- `internal/usecase` — бизнес-логика
- `internal/domain` — сущности и интерфейсы
- `internal/infrastructure` — PostgreSQL, Redis, config, logger
- `migrations` — SQL миграции
- `e2e` — end-to-end тесты

Это не продакшн-платёжка, а учебный проект, который показывает, как можно сделать нормальный backend с безопасными переводами и базовой архитектурой.
