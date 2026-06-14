package handlers

import (
	"encoding/json"
	"net/http"
	"processing/internal/decimal"

	"github.com/google/uuid"
)

type transferDTO struct {
	Sender_id   uuid.UUID `json:"sender_id"`
	Receiver_id uuid.UUID `json:"receiver_id"`
	Amount      string    `json:"amount"`
}

// Transfer хэндлер для отправки транзакции платежа
// POST /transactions
func (h *handler) Transfer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	key := r.Header.Get("Idempotency-Key")
	var dto transferDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, 400, err, 0)
		return
	}

	amount, err := decimal.NewFromString(dto.Amount)
	if err != nil {
		writeError(w, 500, err, 0)
		return
	}

	if err := h.ts.Transfer(ctx, dto.Sender_id, dto.Receiver_id, key, amount); err != nil {
		writeError(w, 500, err, 1)
		return
	}
	//todo вернуть 200 код и отдать id транзакции
}

type transactionDTO struct {
	UserID         uuid.UUID `json:"user_id"`
	IdempotencyKEY string    `json:"Idempotency-Key"`
}

// получение транзакции по id
// GET /transactions/:id
func (h *handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, err, 0)
		return
	}

	var dto transactionDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, 500, err, 0)
		return
	}
	ctx := r.Context()

	transaction, err := h.ts.GetTransaction(ctx, id, dto.UserID, dto.IdempotencyKEY)
	if err != nil {
		writeError(w, 500, err, 1)
		return
	}

	if err := json.NewEncoder(w).Encode(transaction); err != nil {
		writeError(w, 500, err, 0)
		return
	}
}

// выводит транзакции по фильтрам
// Примеры запросов:
// GET /transactions?sender_id=uuid&from=2024-01-01&limit=20&offset=0
// GET /transactions?receiver_id=uuid&min_amount=1000
func (h *handler) TransactionFilter(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/json")

	var dto transactionDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, 400, err, 0)
		return
	}

	ctx := r.Context()
	query := r.URL.Query()
	filter, err := newTransactionFilter(query)
	if err != nil {
		writeError(w, 400, err, 1)
		return
	}

	transactions, err := h.ts.GetTransactionFilter(ctx, filter, dto.UserID, dto.IdempotencyKEY)
	if err != nil {
		writeError(w, 500, err, 1)
		return
	}

	if err := json.NewEncoder(w).Encode(transactions); err != nil {
		writeError(w, 500, err, 0)
		return
	}
}
