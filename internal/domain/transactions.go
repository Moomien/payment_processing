package domain

import (
	"context"
	"fmt"
	"processing/internal/decimal"
	"time"

	"github.com/google/uuid"
)

type TransactionUsecase interface {
	Transfer(ctx context.Context, sender_id, receiver_id uuid.UUID, key string, amount decimal.Decimal) (string, error)
	GetTransaction(ctx context.Context, transactionID, userID uuid.UUID, key string) (Transaction, error)
	GetTransactionFilter(ctx context.Context, t *TransactionFilter, userID uuid.UUID, key string) ([]Transaction, error)
}

type Transaction struct {
	ID          uuid.UUID         `json:"-"`
	Amount      decimal.Decimal   `json:"amount"`
	Sender_id   uuid.UUID         `json:"sender_id"`
	Receiver_id uuid.UUID         `json:"receiver_id"`
	Status      TransactionStatus `json:"status"`
	Created_at  time.Time         `json:"created_at"`
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

type TransactionFilter struct {
	AccountID  uuid.UUID
	SenderID   uuid.UUID
	ReceiverID uuid.UUID
	MinAmount  string
	MaxAmount  string
	From       time.Time
	To         time.Time
	Limit      int
	Offset     int
}
