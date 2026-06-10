package handlers

import (
	"encoding/json"
	"net/http"
	"processing/internal/decimal"
	"processing/internal/usecase"

	"github.com/google/uuid"
)

type handler struct {
	service *usecase.TransferService
}

func NewHandler(transferService *usecase.TransferService) *handler {
	return &handler{service: transferService}
}

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
	//TODO: парсинг какой то dto для transfer
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

	if err := h.service.Transfer(ctx, dto.Sender_id, dto.Receiver_id, key, amount); err != nil {
		writeError(w, 500, err, 1)
		return
	}
}

type transactionDTO struct {
	UserID         uuid.UUID `json:"user_id"`
	IdempotencyKEY string    `json:"Idempotency-Key"`
}

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

	transaction, err := h.service.GetTransaction(ctx, id, dto.UserID, dto.IdempotencyKEY)
	if err != nil {
		writeError(w, 500, err, 1)
		return
	}

	if err := json.NewEncoder(w).Encode(transaction); err != nil {
		writeError(w, 500, err, 0)
		return
	}
}
