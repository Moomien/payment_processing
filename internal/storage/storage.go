package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"processing/internal/decimal"
	transfer "processing/internal/service"
	"time"

	"github.com/google/uuid"
)

type TransactionStorage interface {
	Transaction(ctx context.Context, tx *Transaction) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status transfer.TransactionStatus) error
}

type AccountsStorage interface {
	Create(ctx context.Context, ac *Account) error
	GetById(ctx context.Context, id uuid.UUID) (*Account, error)
}

type Transaction struct {
	ID          uuid.UUID
	Amount      decimal.Decimal
	Sender_id   uuid.UUID
	Receiver_id uuid.UUID
	Status      transfer.TransactionStatus
	Created_at  time.Time
	Updated_at  time.Time
}

func NewTransaction(amount decimal.Decimal, sender_id uuid.UUID, receiver_id uuid.UUID) (*Transaction, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("создание uuid: %w", err)
	}
	return &Transaction{
		ID:          id,
		Amount:      amount,
		Sender_id:   sender_id,
		Receiver_id: receiver_id,
	}, nil
}

type Account struct {
	ID      uuid.UUID
	Name    string
	Balance decimal.Decimal
}

func NewAccount(name string, balance decimal.Decimal) (*Account, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("создание uuid: %w", err)
	}
	return &Account{ID: id, Name: name, Balance: balance}, nil
}

type storage struct {
	db *sql.DB
	tx *sql.Tx
}

func NewStorage(db *sql.DB, tx *sql.Tx) *storage {
	return &storage{db: db, tx: tx}
}

// Create - создаёт аккаунт и возвращает ID
func (s *storage) Create(ctx context.Context, ac *Account) error {
	query := `INSERT INTO accounts(id, name, balance) VALUES($1, $2, $3)`
	if _, err := s.tx.ExecContext(ctx, query, ac.ID, ac.Name, ac.Balance); err != nil {
		return fmt.Errorf("создание аккакунта: %w", err)
	}
	return nil
}

// GetById - возвращает аккаунт по id
func (s *storage) GetById(ctx context.Context, id uuid.UUID) (*Account, error) {
	ac := &Account{}
	query := `SELECT * FROM accounts WHERE id = $1`
	err := s.tx.QueryRowContext(ctx, query, id).Scan(&ac.ID, &ac.Name, &ac.Balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("аккаунт не найден: %w", err)
		}
		return nil, fmt.Errorf("получение данных аккаунта по id: %w", err)
	}
	return ac, nil
}

// Transaction создает транзакцию в бд
func (s *storage) Transaction(ctx context.Context, tx *Transaction) error {
	query := `
	INSERT INTO transactions(id, amount, sender_id, receiver_id) VALUES($1, $2, $3, $4)
	RETURNING status, created_at, updated_at
	`
	if err := s.tx.QueryRowContext(ctx, query, tx.ID, tx.Amount, tx.Sender_id, tx.Receiver_id).
		Scan(&tx.Status, &tx.Created_at, &tx.Updated_at); err != nil {
		return fmt.Errorf("создание транзакции: %w", err)
	}
	return nil
}

// UpdateStatus обновляет статус транзакции в бд
func (s *storage) UpdateStatus(ctx context.Context, tx *Transaction, status transfer.TransactionStatus) error {
	query := `UPDATE transactions SET status = $1 WHERE id = $2
	RETURNING status, created_at, updated_at
	`
	if err := s.tx.QueryRowContext(ctx, query, status, tx.ID).Scan(&tx.Status, &tx.Created_at, &tx.Updated_at); err != nil {
		return fmt.Errorf("обновление статуса транзакции: %w", err)
	}
	return nil
}
