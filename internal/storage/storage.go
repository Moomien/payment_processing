package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"processing/internal/decimal"

	"time"

	"github.com/google/uuid"
)

type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusCompleted TransactionStatus = "completed"
	StatusFailed    TransactionStatus = "failed"
)

type TransactionStorage interface {
	Transaction(ctx context.Context, tx *Transaction) error
	UpdateStatus(ctx context.Context, tx *Transaction, status TransactionStatus) error
}

type AccountsStorage interface {
	Create(ctx context.Context, ac *Account) error
	GetById(ctx context.Context, id uuid.UUID) (*Account, error)
	Sub(ctx context.Context, sender_id uuid.UUID, amount decimal.Decimal) error
	Add(ctx context.Context, receiver_id uuid.UUID, amount decimal.Decimal) error
}

type Transaction struct {
	ID          uuid.UUID
	Amount      decimal.Decimal
	Sender_id   uuid.UUID
	Receiver_id uuid.UUID
	Status      TransactionStatus
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

// паттерн Unit Of Work
type UnitOfWork interface {
	Accounts() AccountsStorage
	Transactions() TransactionStorage
	Commit() error
	Rollback() error
}

type UoWFactory interface {
	NewUoW(ctx context.Context) (UnitOfWork, error)
}

type accountRepo struct{ tx *sql.Tx }
type txRepo struct{ tx *sql.Tx }

type sqlUoW struct {
	tx       *sql.Tx
	accounts *accountRepo
	txs      *txRepo
}

func (u *sqlUoW) Accounts() AccountsStorage        { return u.accounts }
func (u *sqlUoW) Transactions() TransactionStorage { return u.txs }
func (u *sqlUoW) Commit() error                    { return u.tx.Commit() }
func (u *sqlUoW) Rollback() error                  { return u.tx.Rollback() }

type uowFactory struct{ db *sql.DB }

func NewUoWFactory(db *sql.DB) UoWFactory { return &uowFactory{db: db} }

func (u *uowFactory) NewUoW(ctx context.Context) (UnitOfWork, error) {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("tx begin: %w", err)
	}
	return &sqlUoW{
		tx:       tx,
		accounts: &accountRepo{tx: tx},
		txs:      &txRepo{tx: tx},
	}, nil
}

// Create - создаёт аккаунт и возвращает ID
func (s *accountRepo) Create(ctx context.Context, ac *Account) error {
	query := `INSERT INTO accounts(id, name, balance) VALUES($1, $2, $3)`
	if _, err := s.tx.ExecContext(ctx, query, ac.ID, ac.Name, ac.Balance); err != nil {
		return fmt.Errorf("создание аккакунта: %w", err)
	}
	return nil
}

// GetById - возвращает аккаунт по id
func (s *accountRepo) GetById(ctx context.Context, id uuid.UUID) (*Account, error) {
	ac := &Account{}
	query := `SELECT id, name, balance FROM accounts WHERE id = $1`
	err := s.tx.QueryRowContext(ctx, query, id).Scan(&ac.ID, &ac.Name, &ac.Balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("аккаунт не найден: %w", err)
		}
		return nil, fmt.Errorf("получение данных аккаунта по id: %w", err)
	}
	return ac, nil
}

// Sub - вычетает сумму с баланса аккаунта
func (s *accountRepo) Sub(ctx context.Context, sender_id uuid.UUID, amount decimal.Decimal) error {
	query := `UPDATE accounts SET balance = balance - $1 WHERE id = $2`
	if _, err := s.tx.ExecContext(ctx, query, amount, sender_id); err != nil {
		return fmt.Errorf("вычет суммы с баланса: %w", err)
	}
	return nil
}

func (s *accountRepo) Add(ctx context.Context, receiver_id uuid.UUID, amount decimal.Decimal) error {
	query := `UPDATE accounts SET balance = balance + $1 WHERE id = $2`
	if _, err := s.tx.ExecContext(ctx, query, amount, receiver_id); err != nil {
		return fmt.Errorf("добавление суммы на баланс: %w", err)
	}
	return nil
}

// Transaction создает транзакцию в бд
func (s *txRepo) Transaction(ctx context.Context, tx *Transaction) error {
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

// UpdateStatus обновляет статус транзакции в бд и отдает status, created_at, updated_at
func (s *txRepo) UpdateStatus(ctx context.Context, tx *Transaction, status TransactionStatus) error {
	query := `UPDATE transactions SET status = $1 WHERE id = $2
	RETURNING status, created_at, updated_at
	`
	if err := s.tx.QueryRowContext(ctx, query, status, tx.ID).Scan(&tx.Status, &tx.Created_at, &tx.Updated_at); err != nil {
		return fmt.Errorf("обновление статуса транзакции: %w", err)
	}
	return nil
}
