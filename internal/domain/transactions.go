package domain

import (
	"context"
	"fmt"
	"processing/internal/decimal"
	"time"

	"github.com/google/uuid"
)

const MaxIdempotencyKeyLength = 128

type IdempotencyStatus string

const (
	IdempotencyStatusProcessing IdempotencyStatus = "processing"
	IdempotencyStatusCompleted  IdempotencyStatus = "completed"
)

type TransferIdempotency struct {
	SenderID           uuid.UUID
	Key                string
	RequestFingerprint string
	Status             IdempotencyStatus
	TransactionID      *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func ValidateIdempotencyKey(key string) error {
	if key == "" {
		return ErrIdempotencyKeyRequired
	}
	if len(key) > MaxIdempotencyKeyLength {
		return ErrInvalidIdempotencyKey
	}

	for i := 0; i < len(key); i++ {
		char := key[i]
		allowed := char >= 'a' && char <= 'z' ||
			char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' ||
			char == '.' || char == '_' || char == ':' || char == '-'
		if !allowed {
			return ErrInvalidIdempotencyKey
		}
	}

	return nil
}

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
