package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	appLogger, err := logger.NewLogger(cfg.LogLevel, cfg.Environment)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	db, err := sql.Open("pgx", cfg.Postgres.PostgresDSN())
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			appLogger.Error("close postgres", "err", err)
		}
	}()
	db.SetMaxOpenConns(cfg.Postgres.MaxConns)
	db.SetMaxIdleConns(cfg.Postgres.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Postgres.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.Postgres.ConnMaxIdleTime)

	cacheClient := cache.NewRedis(cache.NewRedisOptions{
		Addr:          cfg.Redis.Addr(),
		Username:      cfg.Redis.USER,
		Password:      cfg.Redis.PASSWORD,
		DB:            cfg.Redis.DB,
		RateLimitMin:  cfg.Ratelimit.PerMinute,
		RateLimitHour: cfg.Ratelimit.PerHour,
		RateLimitDay:  cfg.Ratelimit.PerDay,
	})
	defer func() {
		if err := cacheClient.Close(); err != nil {
			appLogger.Error("close redis", "err", err)
		}
	}()

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.PingContext(startupCtx); err != nil {
		startupCancel()
		return fmt.Errorf("ping postgres: %w", err)
	}
	if err := cacheClient.Ping(startupCtx); err != nil {
		startupCancel()
		return fmt.Errorf("ping redis: %w", err)
	}
	startupCancel()

	tx := storage.NewUoWFactory(db)
	jwtManager := jwtLayer.NewManager(cfg.JWT)
	transactionService := usecase.NewTransactionsService(tx, cacheClient, appLogger)
	accountsService := usecase.NewAccountService(tx, cacheClient, appLogger)
	authService := usecase.NewAuthService(tx, cacheClient, appLogger, jwtManager)
	handler := handlers.NewHandler(transactionService, accountsService, authService, appLogger)
	auth := middleware.NewAuth(jwtManager)

	router := http.NewServeMux()
	router.HandleFunc("GET /health", handlers.Health)
	router.HandleFunc("GET /health/live", handlers.Health)
	router.Handle("GET /health/ready", handlers.Readiness(db.PingContext, cacheClient.Ping))
	router.HandleFunc("POST /auth/register", handler.Register)
	router.HandleFunc("POST /auth/login", handler.Login)
	router.HandleFunc("POST /auth/refresh", handler.Refresh)
	router.Handle("POST /auth/logout", auth.Middleware(http.HandlerFunc(handler.Logout)))
	router.Handle("POST /auth/logout-all", auth.Middleware(http.HandlerFunc(handler.LogoutAll)))
	router.Handle("GET /accounts/{id}", auth.Middleware(http.HandlerFunc(handler.GetAccount)))
	router.Handle("GET /accounts/{id}/transactions", auth.Middleware(http.HandlerFunc(handler.AccountTransactions)))
	router.Handle("POST /transactions", auth.Middleware(http.HandlerFunc(handler.Transfer)))
	router.Handle("GET /transactions/{id}", auth.Middleware(http.HandlerFunc(handler.GetTransaction)))

	server := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()
	appLogger.Info("Server started", "server port", cfg.HTTP.Port)

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-signalCtx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server stop: %w", err)
	}
	appLogger.Info("server stopped")
	return nil
}
