package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"processing/internal/domain"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func newTransactionFilter(query url.Values) (*domain.TransactionFilter, error) {
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
	t, err := time.Parse("2006-01-02", query.Get(key))
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

func status(id int) string {
	m := map[int]string{
		200: "OK",
		400: "StatusBadRequest",
		401: "Unauthorized",
		403: "Forbidden",
		404: "StatusNotFound",
		429: "too many requests",
		500: "internal server error",
	}
	value, _ := m[id]
	return value
}

// writeError пишет ошибку клиенту.
// flag:  1 - полная ошибка, любой другой - только часть
func writeError(w http.ResponseWriter, code int, err error, flag int) {
	if flag == 1 {
		json.NewEncoder(w).Encode(map[string]string{"error": status(code)})
		json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
	}
	json.NewEncoder(w).Encode(map[string]string{"error": status(code)})
}

func writeJSON(w http.ResponseWriter, code int, v any) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := buf.WriteTo(w)
	return err
}

func readJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
