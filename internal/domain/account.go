package domain

import (
	"fmt"
	"processing/internal/decimal"

	"github.com/google/uuid"
)

type Account struct {
	ID      uuid.UUID       `json:"account_id"`
	Name    string          `json:"name"`
	Balance decimal.Decimal `json:"balance"`
}

func NewAccount(name string, balance decimal.Decimal) (*Account, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("создание uuid: %w", err)
	}
	return &Account{ID: id, Name: name, Balance: balance}, nil
}
