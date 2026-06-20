package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"

	handlers "processing/internal/delivery/http"
	"processing/internal/delivery/http/middleware"
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
	stor, err := os.OpenFile("storage.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	defer stor.Close()

	redis, err := os.OpenFile("redis.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	defer redis.Close()

	transaction, err := os.OpenFile("transactions_service.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	defer transaction.Close()

	accounts, err := os.OpenFile("accounts_serivce.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	defer accounts.Close()

	auth, err := os.OpenFile("auth_service.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	defer auth.Close()

	h, err := os.OpenFile("handler.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	defer h.Close()

	storagelog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, stor), nil))
	redislog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, redis), nil))
	transactionlog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, transaction), nil))
	accountlog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, accounts), nil))
	authlog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, auth), nil))
	handlerlog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, h), nil))

	db, err := sql.Open("pgx", db_url)
	if err != nil {
		fmt.Println("не получилось подключиться к бд:", err)
		return err
	}

	if err := db.Ping(); err != nil {
		fmt.Println("не получилось пингануть бд:", err)
		return err
	}
	slog.Info("Успешное подключение к бд!")

	tx := storage.NewUoWFactory(db, storagelog)
	cache := cache.NewRedis(redis_url, redislog)
	//kafka

	transactionService := usecase.NewTransactionsService(tx, cache, transactionlog)
	accountsService := usecase.NewAccountService(tx, cache, accountlog)
	authService := usecase.NewAuthService(tx, cache, authlog)

	handler := handlers.NewHandler(transactionService, accountsService, authService, handlerlog)

	router := http.NewServeMux()

	router.HandleFunc("POST /auth/register", handler.Register)
	router.HandleFunc("POST /auth/login", handler.Login)
	router.HandleFunc("POST /auth/refresh", handler.Refresh)

	router.Handle("POST /auth/logout", middleware.AuthMiddleware(http.HandlerFunc(handler.Logout)))
	router.Handle("POST /auth/logout-all", middleware.AuthMiddleware(http.HandlerFunc(handler.LogoutAll)))
	router.Handle("GET /accounts/{id}", middleware.AuthMiddleware(http.HandlerFunc(handler.GetAccount)))
	router.Handle("GET /accounts/{id}/transactions", middleware.AuthMiddleware(http.HandlerFunc(handler.AccountTransactions)))
	router.Handle("POST /transactions", middleware.AuthMiddleware(http.HandlerFunc(handler.Transfer)))
	router.Handle("GET /transactions/{id}", middleware.AuthMiddleware(http.HandlerFunc(handler.GetTransaction)))

	log.Println("сервер запущен на :8080!")
	http.ListenAndServe(":8080", router)

	return nil
}
