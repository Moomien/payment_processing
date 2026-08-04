POSTGRES_PORT ?= 5432
DB_URL ?= postgres://admin:secret@localhost:$(POSTGRES_PORT)/postgres_bd?sslmode=disable
GO ?= go

.PHONY: run test lint e2e compose-up compose-down docker-up docker-down migrate-up migrate-down

run:
	$(GO) run ./cmd/server

test:
	$(GO) test ./internal/...

lint:
	$(GO) vet ./...

e2e:
	$(GO) test -count=1 ./e2e/...

compose-up:
	docker compose up --build -d --remove-orphans

compose-down:
	docker compose down --remove-orphans

docker-up: compose-up

docker-down: compose-down

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down
