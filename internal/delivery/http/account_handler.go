package handlers

import (
	"encoding/json"
	"net/http"
	"processing/internal/decimal"
	"processing/internal/domain"
)

type AccountDTO struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// POST /accounts
func (h *handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	var dto AccountDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, 400, err, 1)
		return
	}

	balance, err := decimal.NewFromString("0")
	if err != nil {
		writeError(w, 500, err, 0)
		return
	}

	acc, err := domain.NewAccount(dto.Name, balance)
	if err != nil {
		writeError(w, 500, err, 0)
		return
	}

	if err := h.as.Create(ctx, acc, r.RemoteAddr); err != nil {
		writeError(w, 500, err, 1)
		return
	}
}
