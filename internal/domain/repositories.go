package domain

import (
	"context"
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

// Интерфейсы репозиториев (контракты для infrastructure слоя)
type TransactionStorage interface {
	Transaction(ctx context.Context, tx *Transaction) error
	UpdateStatus(ctx context.Context, tx *Transaction, status TransactionStatus) error
	GetByID(ctx context.Context, transactionID uuid.UUID) (Transaction, error)
}

type AccountsStorage interface {
	Create(ctx context.Context, ac *Account) error
	GetById(ctx context.Context, id uuid.UUID) (*Account, error)
	Sub(ctx context.Context, sender_id uuid.UUID, amount decimal.Decimal) error
	Add(ctx context.Context, receiver_id uuid.UUID, amount decimal.Decimal) error
}

// UnitOfWork управляет аккаунтами, транзакциями и транзакциями самой бд
type UnitOfWork interface {
	Accounts() AccountsStorage
	Transactions() TransactionStorage
	Commit() error
	Rollback() error
}

// TxUOW нужен для создания транзакции (фабрика)
type TxUOW interface {
	NewTX(ctx context.Context) (UnitOfWork, error)
}

// Доменные сущности
type Transaction struct {
	ID          uuid.UUID         `json:"-"`
	Amount      decimal.Decimal   `json:"amount"`
	Sender_id   uuid.UUID         `json:"sender_id"`
	Receiver_id uuid.UUID         `json:"receiver_id"`
	Status      TransactionStatus `json:"status"`
	Created_at  time.Time         `json:"created_at"`
}

type Account struct {
	ID      uuid.UUID       `json:"account_id"`
	Name    string          `json:"name"`
	Balance decimal.Decimal `json:"balance"`
}

// Фабричные методы для создания доменных сущностей
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

func NewAccount(name string, balance decimal.Decimal) (*Account, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("создание uuid: %w", err)
	}
	return &Account{ID: id, Name: name, Balance: balance}, nil
}
