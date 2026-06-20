package storage

import (
	"context"
	"fmt"
	"processing/internal/decimal"
	"processing/internal/domain"

	"github.com/google/uuid"
)

func sqlrequest(ctx context.Context, filter domain.TransactionFilter) (string, []interface{}) {
	query := `SELECT id, amount, sender_id, receiver_id, status, created_at FROM transactions WHERE 1=1`
	args := []interface{}{}
	argCounter := 1

	if filter.AccountID != uuid.Nil {
		query += fmt.Sprintf(" AND(receiver_id = $%d OR sender_id = $%d)", argCounter, argCounter)
		args = append(args, filter.AccountID)
		argCounter++
	}

	if filter.SenderID != uuid.Nil {
		query += fmt.Sprintf(" AND sender_id = $%d", argCounter)
		args = append(args, filter.SenderID)
		argCounter++
	}

	if filter.ReceiverID != uuid.Nil {
		query += fmt.Sprintf(" AND receiver_id = $%d", argCounter)
		args = append(args, filter.ReceiverID)
		argCounter++
	}

	if filter.MinAmount != "" {
		minAmount, err := decimal.NewFromString(filter.MinAmount)
		if err != nil {
			return "", nil
		}
		query += fmt.Sprintf(" AND amount >= $%d", argCounter)
		args = append(args, minAmount)
		argCounter++
	}

	if filter.MaxAmount != "" {
		maxAmount, err := decimal.NewFromString(filter.MaxAmount)
		if err != nil {
			return "", nil
		}
		query += fmt.Sprintf(" AND amount <= $%d", argCounter)
		args = append(args, maxAmount)
		argCounter++
	}

	if !filter.From.IsZero() {
		query += fmt.Sprintf(" AND created_at >= $%d", argCounter)
		args = append(args, filter.From)
		argCounter++
	}

	if !filter.To.IsZero() {
		query += fmt.Sprintf(" AND created_at <= $%d", argCounter)
		args = append(args, filter.To)
		argCounter++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCounter)
		args = append(args, filter.Limit)
		argCounter++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCounter)
		args = append(args, filter.Offset)
	}

	return query, args
}
