package handlers

import (
	"net/http"
	"processing/internal/domain"
	"strconv"
)

type AccountDTO struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Email    string `json:"email"`
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

	if err := writeJSON(w, http.StatusOK, account); err != nil {
		h.log.Error("[GetAccount] json encode", "err", err)
	}
}

type AccountTransactions struct {
	Slice []domain.Transaction `json:"transactions"`
	Total int                  `json:"pages"`
}

// Get /accounts/:id/transactions?limit=..&offset=...
func (h *handler) AccountTransactions(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	id, err := parseUUID(r.URL.Query(), "id")
	if err != nil {
		writeError(w, 500, err, 0)
		return
	}

	limit := r.URL.Query().Get("limit")
	offset := r.URL.Query().Get("offset")
	l, err := strconv.Atoi(limit)
	if err != nil {
		h.log.Error("strconv ", "err", err)
		return
	}

	o, err := strconv.Atoi(offset)
	if err != nil {
		h.log.Error("strconv ", "err", err)
		writeError(w, 500, err, 0)
		return
	}

	total, transactions, err := h.as.TransactionHistory(ctx, id, l, o)
	if err != nil {
		writeError(w, 500, err, 0)
		return
	}

	dto := AccountTransactions{
		Slice: transactions,
		Total: total,
	}

	if err := writeJSON(w, http.StatusOK, dto); err != nil {
		h.log.Error("[AccountTransactions] json encode", "err", err)
	}
}
