package domain

import (
	"context"
	"fmt"
	"processing/internal/decimal"

	"github.com/google/uuid"
)

type Account struct {
	ID      uuid.UUID       `json:"account_id"`
	Name    string          `json:"name"`
	Email   string          `json:"email"`
	Balance decimal.Decimal `json:"balance"`
}

func NewAccount(name string, balance decimal.Decimal) (*Account, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("создание uuid: %w", err)
	}
	return &Account{ID: id, Name: name, Balance: balance}, nil
}

type AccountsUsecase interface {
	Create(ctx context.Context, acc *Account, ip string) error
	GetAccount(ctx context.Context, id uuid.UUID) (*Account, error)
	TransactionHistory(ctx context.Context, accountID uuid.UUID, limit, offset int) (int, []Transaction, error)
}
