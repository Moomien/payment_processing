package main

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"os"
	"processing/internal/handlers"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const db_url = "postgres://admin:secret@localhost:5432/postgres_bd"

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	file, err := os.OpenFile("log.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), nil))
	slog.SetDefault(logger)
	slog.Info("создан логгер")

	db, err := sql.Open("pgx", db_url)
	if err != nil {
		logger.Error("не получилось подключиться к бд", "err", err)
		return err
	}
	if err := db.Ping(); err != nil {
		logger.Error("не получилось пингануть бд", "err", err)
		return err
	}
	slog.Info("Успешное подключение к бд!")
	handler := handlers.NewHandler(db)
	mux := http.NewServeMux()
	mux.HandleFunc("/transactions", handler.Transfer)
	slog.Info("сервер запущен на :8080")
	http.ListenAndServe(":8080", mux)
	return nil
}
