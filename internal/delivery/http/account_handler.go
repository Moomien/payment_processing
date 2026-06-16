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

// CreateAccount создаёт аккаунт пользователя и возврат данных о нём клиенту
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(acc)
}

// выводит информацию об аккаунте по айди
// GET /accounts/:id
func (h *handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	id, err := parseUUID(r.URL.Query(), "id")
	if err != nil {
		writeError(w, 500, err, 0)
		return
	}

	account, err := h.as.GetAccount(ctx, id)
	if err != nil {
		writeError(w, 500, err, 1)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(account); err != nil {
		writeError(w, 500, err, 1)
		return
	}
}

//Get /accounts/:id/transactions?limit=..&offset=...
func(h *handler) AccountTransactions(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	id, err := parseUUID(r.URL.Query(), "id")
	if err != nil {
		writeError(w, 500, err, 0)
		return
	}
	limit := 
	if err := h.as.TransactionHistory(ctx, id, )

}