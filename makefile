POSTGRES_PORT ?= 5432
DB_URL ?= postgres://admin:secret@localhost:$(POSTGRES_PORT)/postgres_bd?sslmode=disable
GO ?= go

.PHONY: run stop test lint e2e compose-up compose-down docker-up docker-down migrate-up migrate-down

run:
	$(GO) run ./cmd/server

stop:
	docker compose stop app

test:
	$(GO) test ./internal/...

lint:
	$(GO) vet ./...

e2e:
	$(GO) test -count=1 ./e2e/...

docker-up:
	docker compose up --build -d --remove-orphans

docker-down:
	docker compose down --remove-orphans

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down
