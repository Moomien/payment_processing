# Payment Processing

Учебный backend-сервис для переводов между внутренними счетами. Он показывает
атомарное изменение балансов, PostgreSQL-идемпотентность, конкурентную работу с
деньгами, JWT-сессии и ограничение частоты запросов.

> Это технический демонстрационный проект, а не система для реальных платежей.
> В нём нет валют, бухгалтерского ledger, внешнего эквайринга и регуляторного контура.

## Быстрый запуск

Нужны Docker Engine и Docker Compose:

```bash
docker compose up --build
```

Compose поднимает PostgreSQL и Redis, применяет миграции, один раз создаёт два
демонстрационных счёта и запускает API на `http://localhost:8080`.

Проверка готовности:

```bash
curl http://localhost:8080/health/ready
```

Демонстрационные пользователи существуют только в `ENVIRONMENT=development`:

| Пользователь | Email | Пароль | ID счёта | Начальный баланс |
|---|---|---|---|---|
| Отправитель | `sender@example.test` | `DemoPass123!` | `11111111-1111-1111-1111-111111111111` | `1000.00` |
| Получатель | `receiver@example.test` | `DemoPass123!` | `22222222-2222-2222-2222-222222222222` | `1000.00` |

Сначала получите токены:

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"sender@example.test","password":"DemoPass123!"}'
```

Затем передайте `access_token` как Bearer token:

```bash
curl -X POST http://localhost:8080/transactions \
  -H "Authorization: Bearer <access_token>" \
  -H "Idempotency-Key: demo-transfer-1" \
  -H "Content-Type: application/json" \
  -d '{"receiver_id":"22222222-2222-2222-2222-222222222222","amount":"125.50"}'
```

Повтор этого запроса с тем же ключом вернёт тот же `transaction_id` без второго
списания. Тот же ключ с другим payload вернёт `409 Conflict`.

## Гарантии перевода

- PostgreSQL является source of truth для балансов, операций и идемпотентности.
- Списание, зачисление, запись операции и результат idempotency key фиксируются
  одной SQL-транзакцией.
- Списание выполняется условным `UPDATE ... WHERE balance >= amount`; БД также
  запрещает отрицательный баланс, неположительную сумму и перевод самому себе.
- Счета блокируются в стабильном UUID-порядке, поэтому встречные переводы не
  захватывают строки в противоположном порядке.
- Используется явно выбранный уровень `READ COMMITTED`. Ошибки PostgreSQL
  `serialization_failure` и `deadlock_detected` повторяются ограниченно с backoff.
- Суммы должны точно помещаться в `NUMERIC(36,18)`; precision, scale и exponent
  проверяются до SQL.
- Redis используется для rate limiting. При недоступности Redis денежная операция
  отклоняется (fail closed), но её idempotency state и балансы откатываются.

## API

| Метод | Маршрут | Назначение | Авторизация |
|---|---|---|---|
| `POST` | `/auth/register` | Регистрация | Нет |
| `POST` | `/auth/login` | Вход | Нет |
| `POST` | `/auth/refresh` | Ротация refresh token | Refresh token |
| `POST` | `/auth/logout` | Отзыв текущей сессии | Bearer token |
| `POST` | `/auth/logout-all` | Отзыв всех refresh-сессий | Bearer token |
| `GET` | `/accounts/{id}` | Свой счёт | Bearer token |
| `GET` | `/accounts/{id}/transactions` | История счёта | Bearer token |
| `POST` | `/transactions` | Перевод | Bearer token |
| `GET` | `/transactions/{id}` | Операция по ID | Bearer token |
| `GET` | `/health/live` | Liveness | Нет |
| `GET` | `/health/ready` | PostgreSQL/Redis readiness | Нет |

Денежные значения передаются JSON-строками. JSON body ограничен 64 KiB;
неизвестные поля и несколько JSON-объектов отклоняются.

## Архитектура

```text
cmd/server                  composition root и HTTP-сервер
cmd/migrate                 запуск Goose-миграций
cmd/seed                    development-only demo seed
internal/delivery/http      handlers, DTO и middleware
internal/usecase            прикладные сценарии
internal/domain             сущности и контракты
internal/infrastructure     PostgreSQL, Redis, config и logging
migrations                  версионированная схема PostgreSQL
e2e                         Testcontainers end-to-end тесты
```

Usecase-слой зависит от интерфейсов хранилища и cache. SQL-транзакция управляется
через Unit of Work. Redis не участвует в фиксации денежного результата.

## Конфигурация

Локальный пример находится в `.env.example`; настоящий `.env` игнорируется Git.
Основные переменные:

| Группа | Переменные |
|---|---|
| HTTP | `HTTP_PORT`, `HTTP_READ_HEADER_TIMEOUT`, `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`, `HTTP_SHUTDOWN_TIMEOUT` |
| PostgreSQL | `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_SSLMODE`, `POSTGRES_MAX_CONNS`, `POSTGRES_MAX_IDLE_CONNS`, `POSTGRES_CONN_MAX_LIFETIME`, `POSTGRES_CONN_MAX_IDLE_TIME` |
| Redis | `REDIS_HOST`, `REDIS_PORT`, `REDIS_USER`, `REDIS_PASSWORD`, `REDIS_DB` |
| JWT | `ACCESS_TOKEN_SECRET`, `REFRESH_TOKEN_SECRET`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `ISSUER` |
| Limits | `RATE_LIMIT_PER_MINUTE`, `RATE_LIMIT_PER_HOUR`, `RATE_LIMIT_PER_DAY` |

Production-конфигурация запрещает PostgreSQL без строгого SSL и Redis без пароля.

## Проверки

```bash
go fmt ./...
go vet ./...
go test ./internal/...
go build ./cmd/...
```

E2E требуют работающий Docker Engine:

```bash
go test -count=1 ./e2e/...
```

CI выполняет vet и быстрые тесты на Linux/Windows, а e2e — на Linux runner с Docker.
Конкурентный e2e проверяет встречные переводы, повтор одного idempotency key и
сохранение общей суммы денег.

## Осознанные ограничения

- модель синхронная и поддерживает одну неявную денежную единицу;
- баланс хранится как агрегат, полноценного double-entry ledger нет;
- demo seed — доверенная development-only операция, а не пользовательский deposit;
- access token остаётся действительным до истечения TTL после logout-all;
- Redis rate limits приблизительны между несколькими временными окнами;
- для истории пока используется offset pagination.
