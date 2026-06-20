package main

import (
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"

	handlers "processing/internal/delivery/http"
	"processing/internal/delivery/http/middleware"
	"processing/internal/infrastructure/cache"
	"processing/internal/infrastructure/config"
	"processing/internal/infrastructure/logger"
	"processing/internal/infrastructure/storage"
	"processing/internal/usecase"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger, err := logger.NewLogger(cfg.LogLevel, cfg.Environment)
	if err != nil {
		panic(err)
	}

	postgres_url := cfg.Postgres.PostgresDSN()
	db, err := sql.Open("pgx", postgres_url)
	if err != nil {
		logger.Debug("не получилось подключиться к бд", "err", err)
		return err
	}
	if err := db.Ping(); err != nil {
		logger.Debug("не получилось пингануть бд", "err", err)
		return err
	}
	slog.Info("Успешное подключение к бд!")

	redis_url := cfg.Redis.RedisDSN()
	cache := cache.NewRedis(cache.NewRedisOptions{
		Addr:          redis_url,
		RateLimitMin:  cfg.Redis.RateLimitMin,
		RateLimitHour: cfg.Redis.RateLimitHour,
		RateLimitDay:  cfg.Redis.RateLimitDay,
	})
	//kafka
	////
	////

	tx := storage.NewUoWFactory(db)
	transactionService := usecase.NewTransactionsService(tx, cache, logger)
	accountsService := usecase.NewAccountService(tx, cache, logger)
	authService := usecase.NewAuthService(tx, cache, logger)

	handler := handlers.NewHandler(transactionService, accountsService, authService, logger)

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
