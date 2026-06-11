package main

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	handlers "processing/internal/delivery/http"
	"processing/internal/infrastructure/cache"
	"processing/internal/infrastructure/storage"
	"processing/internal/usecase"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	db_url    = "postgres://admin:secret@localhost:5432/postgres_bd"
	redis_url = "localhost:6379"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	app, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer app.Close()

	stor, err := os.OpenFile("storage.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer stor.Close()

	redis, err := os.OpenFile("redis.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer redis.Close()

	applog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, app), nil))
	storagelog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, stor), nil))
	redislog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, redis), nil))

	db, err := sql.Open("pgx", db_url)
	if err != nil {
		applog.Error("не получилось подключиться к бд", "err", err)
		return err
	}
	if err := db.Ping(); err != nil {
		applog.Error("не получилось пингануть бд", "err", err)
		return err
	}
	slog.Info("Успешное подключение к бд!")
	tx := storage.NewUoWFactory(db, storagelog)
	cache := cache.NewRedis(redis_url, redislog)
	path := filepath.Join("service.log") // возможно нужно по другому
	transferService := usecase.NewService(tx, cache, path)
	handler := handlers.NewHandler(transferService)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /transactions", handler.Transfer)
	mux.HandleFunc("GET /transactions/{id}", handler.GetTransaction)
	slog.Info("сервер запущен на :8080")
	http.ListenAndServe(":8080", mux)

	return nil
}
