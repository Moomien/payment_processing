POSTGRES_PORT ?= 5432
DB_URL ?= postgres://admin:secret@localhost:$(POSTGRES_PORT)/postgres_bd?sslmode=disable

.PHONY: run stop test lint e2e compose-up compose-down docker-up docker-down migrate-up migrate-down

run:
	go run ./cmd/server

stop:
	docker compose stop app

test:
	go test ./internal/...


## Все эндпоинты: 
	// GET /health
	// GET /health/live
	// GET /health/ready
	// POST /auth/register
	// POST /auth/login
	// POST /auth/refresh
	// POST /auth/logout
	// POST /auth/logout-all
	// GET /accounts/{id}
	// GET /accounts/{id}/transactions

api-test:
	make api-health-test
	make api-auth-test
	make api-accounts-test

api-health-test:
	curl http://localhost:8080/health || (echo "Health check failed" && exit 1)
	curl http://localhost:8080/health/live || (echo "Liveness check failed" && exit 1)
	curl http://localhost:8080/health/ready || (echo "Readiness check failed" && exit 1)

api-auth-test:
	go run ./scripts/api-auth-test/main.go

api-accounts-test:
	go run ./scripts/api-accounts-test/main.go

lint:
	go vet ./...

e2e:
	go test -count=1 ./e2e/...

docker-up:
	docker compose up --build -d --remove-orphans

docker-down:
	docker compose down --remove-orphans

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down

clean:
	docker compose down --remove-orphans
	docker volume rm processing_postgres_datatestpass