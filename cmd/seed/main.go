package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"processing/internal/infrastructure/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

const demoPassword = "DemoPass123!"

var demoAccounts = []struct {
	id      string
	name    string
	email   string
	balance string
}{
	{id: "11111111-1111-1111-1111-111111111111", name: "Demo Sender", email: "sender@example.test", balance: "1000.00"},
	{id: "22222222-2222-2222-2222-222222222222", name: "Demo Receiver", email: "receiver@example.test", balance: "1000.00"},
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if os.Getenv("ENVIRONMENT") != config.EnvironmentDevelopment {
		return errors.New("demo seed is allowed only with ENVIRONMENT=development")
	}
	cfg, err := config.LoadPostgres()
	if err != nil {
		return fmt.Errorf("load postgres config: %w", err)
	}
	db, err := sql.Open("pgx", cfg.PostgresDSN())
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), 12)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer tx.Rollback()
	for _, account := range demoAccounts {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO accounts(id, name, email, password_hash, role, balance)
			VALUES($1, $2, $3, $4, 'user', $5)
			ON CONFLICT DO NOTHING
		`, account.id, account.name, account.email, string(hash), account.balance); err != nil {
			return fmt.Errorf("seed %s: %w", account.email, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit demo seed: %w", err)
	}
	return nil
}
