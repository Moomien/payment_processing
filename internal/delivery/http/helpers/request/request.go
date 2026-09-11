package request

import (
	"fmt"
	"net/url"
	"processing/internal/domain"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func NewTransactionFilter(query url.Values) (*domain.TransactionFilter, error) {
	senderID, err := parseUUID(query, "sender_id")
	if err != nil {
		return nil, err
	}
	receiverID, err := parseUUID(query, "receiver_id")
	if err != nil {
		return nil, err
	}

	minAmount := query.Get("min_amount")
	maxAmount := query.Get("max_amount")

	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil {
		return nil, fmt.Errorf("невалидный limit: %w", err)
	}
	offset, err := strconv.Atoi(query.Get("offset"))
	if err != nil {
		return nil, fmt.Errorf("невалидный offset: %w", err)
	}

	from, err := parseTime(query, "from")
	if err != nil {
		return nil, err
	}

	to, err := parseTime(query, "to")
	if err != nil {
		return nil, err
	}

	return &domain.TransactionFilter{
		SenderID:   senderID,
		ReceiverID: receiverID,
		MinAmount:  minAmount,
		MaxAmount:  maxAmount,
		From:       from,
		To:         to,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

func parseUUID(query url.Values, key string) (uuid.UUID, error) {
	val := query.Get(key)
	if val == "" {
		return uuid.UUID{}, nil
	}

	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("невилдный %s", val)
	}
	return id, nil
}

func parseTime(query url.Values, key string) (time.Time, error) {
	val := query.Get(key)
	if val == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse("2006-01-02", val)
	if err != nil {
		return time.Time{}, fmt.Errorf("невалидная дата %s: %w", key, err)
	}

	return t, nil
}
