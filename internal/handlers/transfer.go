package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"processing/internal/cache"
	"processing/internal/decimal"
	"processing/internal/service"
	"processing/internal/storage"

	"github.com/google/uuid"
)

type handler struct {
	service *service.TransferService
}

func NewHandler(db *sql.DB) *handler {
	factory := storage.NewUoWFactory(db)
	return &handler{service: service.NewService(factory, cache.NewRedis("localhost:6379"))}
}

type transferDTO struct {
	Sender_id   uuid.UUID `json:"sender_id"`
	Receiver_id uuid.UUID `json:"receiver_id"`
	Amount      string    `json:"amount"`
}

// Transfer хэндлер для отправки транзакции платежа
// POST /transfer
func (h *handler) Transfer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	//TODO: парсинг какой то dto для transfer
	var dto transferDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": status(400)})
	}
	amount, err := decimal.NewFromString(dto.Amount)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": status(500)})
	}

	if err := h.service.Transfer(ctx, dto.Sender_id, dto.Receiver_id, amount); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": status(500)})
	}
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
