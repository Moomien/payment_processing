DB_URL = "postgres://admin:secret:@localhost:5432/postgres_db"

docker-up:
	docker compose up -d 

docker-down:
	docker compose down

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down

run:
	go run cmd/main.go