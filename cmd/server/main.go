package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	handlers "processing/internal/delivery/http"
	jwtLayer "processing/internal/delivery/http/jwt"
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

	redis_url := cfg.Redis.Addr()
	cache := cache.NewRedis(cache.NewRedisOptions{
		Addr:          redis_url,
		Username:      cfg.Redis.USER,
		Password:      cfg.Redis.PASSWORD,
		RateLimitMin:  cfg.Redis.RateLimitMin,
		RateLimitHour: cfg.Redis.RateLimitHour,
		RateLimitDay:  cfg.Redis.RateLimitDay,
	})

	tx := storage.NewUoWFactory(db)
	jwtManager := jwtLayer.NewManager(cfg.JWT)
	transactionService := usecase.NewTransactionsService(tx, cache, logger)
	accountsService := usecase.NewAccountService(tx, cache, logger)
	authService := usecase.NewAuthService(tx, cache, logger, jwtManager)

	handler := handlers.NewHandler(transactionService, accountsService, authService, logger)
	auth := middleware.NewAuth(jwtManager)

	router := http.NewServeMux()

	router.HandleFunc("GET /health", handlers.Health)
	router.HandleFunc("POST /auth/register", handler.Register)
	router.HandleFunc("POST /auth/login", handler.Login)
	router.HandleFunc("POST /auth/refresh", handler.Refresh)

	router.Handle("POST /auth/logout", auth.Middleware(http.HandlerFunc(handler.Logout)))
	router.Handle("POST /auth/logout-all", auth.Middleware(http.HandlerFunc(handler.LogoutAll)))
	router.Handle("GET /accounts/{id}", auth.Middleware(http.HandlerFunc(handler.GetAccount)))
	router.Handle("GET /accounts/{id}/transactions", auth.Middleware(http.HandlerFunc(handler.AccountTransactions)))
	router.Handle("POST /transactions", auth.Middleware(http.HandlerFunc(handler.Transfer)))
	router.Handle("GET /transactions/{id}", auth.Middleware(http.HandlerFunc(handler.GetTransaction)))

	slog.Info("сервер запущен", "port", cfg.HTTP.Port)

	if err := http.ListenAndServe(":8080", router); err != nil {
		return fmt.Errorf("http server: %w", err)
	}

	return nil
}
