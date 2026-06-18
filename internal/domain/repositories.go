package domain

import (
	"context"
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
	GetTransactions(ctx context.Context, filter TransactionFilter) ([]Transaction, error)
	TotalTransactions(ctx context.Context, userID uuid.UUID) (int, error)
}

type AccountsStorage interface {
	Create(ctx context.Context, ac *Account) error
	GetById(ctx context.Context, id uuid.UUID) (*Account, error)
	GetByEmail(ctx context.Context, email string) (*Account, error)
	Sub(ctx context.Context, sender_id uuid.UUID, amount decimal.Decimal) error
	Add(ctx context.Context, receiver_id uuid.UUID, amount decimal.Decimal) error
}

// TokenStorage управляет refresh токенами
type TokenStorage interface {
	SaveRefreshToken(ctx context.Context, jti string, user_id string, expires_at time.Time) error
	GetRefreshToken(ctx context.Context, jti string) (*RefreshSession, error)
	RevokeRefreshToken(ctx context.Context, jti string) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

// UnitOfWork управляет аккаунтами, транзакциями и транзакциями самой бд
type UnitOfWork interface {
	Accounts() AccountsStorage
	Transactions() TransactionStorage
	Tokens() TokenStorage
	Commit() error
	Rollback() error
}

type TxUOW interface {
	// TxUOW нужен для создания транзакции (фабрика)
	NewTX(ctx context.Context) (UnitOfWork, error)
}
