package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	handlers "processing/internal/delivery/http"
	"processing/internal/delivery/http/middleware"
	"processing/internal/infrastructure/cache"
	"processing/internal/infrastructure/logger"
	"processing/internal/infrastructure/storage"
	"processing/internal/usecase"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestServer struct {
	Server   *httptest.Server
	DB       *sql.DB
	Logger   *slog.Logger
	Postgres *postgres.PostgresContainer
	Redis    *redis.RedisContainer
}

func SetupTestServer(t *testing.T) *TestServer {
	ctx := context.Background()

	t.Setenv("accessSecretKey", "test-access-secret-key-for-testing-only")
	t.Setenv("refreshSecretKey", "test-refresh-secret-key-for-testing-only")

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("не удалось запустить postgres контейнер: %v", err)
	}

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("не удалось получить connection string: %v", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("не удалось подключиться к БД: %v", err)
	}

	if err := applyMigrations(db); err != nil {
		t.Fatalf("не удалось применить миграции: %v", err)
	}

	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	if err != nil {
		t.Fatalf("не удалось запустить redis контейнер: %v", err)
	}

	redisAddr, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("не удалось получить redis connection string: %v", err)
	}

	if len(redisAddr) > 8 && redisAddr[:8] == "redis://" {
		redisAddr = redisAddr[8:]
	}

	testCache := cache.NewRedis(cache.NewRedisOptions{
		Addr:          redisAddr,
		RateLimitMin:  10000,
		RateLimitHour: 100000,
		RateLimitDay:  1000000,
	})

	testLogger, err := logger.NewLogger("debug", "test")
	if err != nil {
		t.Fatalf("не удалось создать логгер: %v", err)
	}

	tx := storage.NewUoWFactory(db)
	transactionService := usecase.NewTransactionsService(tx, testCache, testLogger)
	accountsService := usecase.NewAccountService(tx, testCache, testLogger)
	authService := usecase.NewAuthService(tx, testCache, testLogger)

	handler := handlers.NewHandler(transactionService, accountsService, authService, testLogger)

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

	server := httptest.NewServer(router)

	t.Cleanup(func() {
		server.Close()
		db.Close()
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("не удалось остановить postgres контейнер: %v", err)
		}
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("не удалось остановить redis контейнер: %v", err)
		}
	})

	return &TestServer{
		Server:   server,
		DB:       db,
		Logger:   testLogger,
		Postgres: postgresContainer,
		Redis:    redisContainer,
	}
}

func applyMigrations(db *sql.DB) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("не удалось установить диалект postgres: %w", err)
	}

	migrationDir := filepath.Join("..", "migrations")

	if err := goose.Up(db, migrationDir); err != nil {
		return fmt.Errorf("не удалось применить миграции: %w", err)
	}

	return nil
}

type TestUser struct {
	AccountID    string
	Email        string
	Username     string
	AccessToken  string
	RefreshToken string
}

func createTestUser(t *testing.T, ts *TestServer, email, password, username string) *TestUser {
	t.Helper()

	registerPayload := map[string]string{
		"email":    email,
		"password": password,
		"username": username,
	}

	body, err := json.Marshal(registerPayload)
	if err != nil {
		t.Fatalf("не удалось сериализовать payload: %v", err)
	}

	resp, err := http.Post(ts.Server.URL+"/auth/register", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("не удалось выполнить запрос регистрации: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("регистрация не удалась, статус: %d", resp.StatusCode)
	}

	var registerResp struct {
		Account struct {
			ID       string `json:"id"`
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"account"`
		Tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		t.Fatalf("не удалось декодировать ответ: %v", err)
	}

	return &TestUser{
		AccountID:    registerResp.Account.ID,
		Email:        registerResp.Account.Email,
		Username:     registerResp.Account.Username,
		AccessToken:  registerResp.Tokens.AccessToken,
		RefreshToken: registerResp.Tokens.RefreshToken,
	}
}
