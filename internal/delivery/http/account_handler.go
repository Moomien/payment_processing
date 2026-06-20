package handlers

import (
	"errors"
	"net/http"
	"processing/internal/domain"
	"strconv"

	"github.com/google/uuid"
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

	accountIDStr := r.PathValue("id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err, 0)
		return
	}

	ctxUserID, ok := ctx.Value("user_id").(string)
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("user_id не найден в контексте"), 0)
		return
	}
	if ctxUserID != accountID.String() {
		writeError(w, http.StatusForbidden, domain.ErrAccessDenied, 0)
		return
	}

	account, err := h.as.GetAccount(ctx, accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, 1)
		return
	}

	if err := writeJSON(w, http.StatusOK, account); err != nil {
		h.log.Error("[GetAccount] json encode", "err", err)
	}
}

type AccountTransactions struct {
	Transactions []domain.Transaction `json:"transactions"`
	Total        int                  `json:"total"`
}

// Get /accounts/:id/transactions?limit=..&offset=...
func (h *handler) AccountTransactions(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	accountIDStr := r.PathValue("id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err, 0)
		return
	}

	ctxUserID, ok := ctx.Value("user_id").(string)
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("user_id не найден в контексте"), 0)
		return
	}
	if ctxUserID != accountID.String() {
		writeError(w, http.StatusForbidden, domain.ErrAccessDenied, 0)
		return
	}

	limit := r.URL.Query().Get("limit")
	offset := r.URL.Query().Get("offset")
	l, err := strconv.Atoi(limit)
	if err != nil {
		l = 10
	}
	o, err := strconv.Atoi(offset)
	if err != nil {
		o = 0
	}

	total, transactions, err := h.as.TransactionHistory(ctx, accountID, l, o)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
		return
	}

	dto := AccountTransactions{
		Transactions: transactions,
		Total:        total,
	}

	if err := writeJSON(w, http.StatusOK, dto); err != nil {
		h.log.Error("[AccountTransactions] json encode", "err", err)
	}
}
