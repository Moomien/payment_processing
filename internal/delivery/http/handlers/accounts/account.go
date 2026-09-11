package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	httpapp "processing/internal/delivery/http/app"
	"processing/internal/delivery/http/helpers/httputil"
	"processing/internal/delivery/http/requestctx"
	"processing/internal/domain"
	"strconv"

	"github.com/google/uuid"
)

type AccountDTO struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type Handler struct {
	as  domain.AccountsUsecase
	log *slog.Logger
}

func New(app *httpapp.App) *Handler {
	return &Handler{
		as:  app.AccountsUsecase,
		log: app.Log,
	}
}

func NewHandler(ts domain.TransactionUsecase, as domain.AccountsUsecase, auth domain.AuthUseCase, log *slog.Logger) *Handler {
	return &Handler{
		as:  as,
		log: log,
	}
}

// выводит информацию об аккаунте по айди
// GET /accounts/:id
func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	accountIDStr := r.PathValue("id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	identity, ok := requestctx.IdentityFrom(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, errors.New("user_id не найден в контексте"), 0)
		return
	}
	if identity.UserID != accountID.String() {
		httputil.WriteError(w, http.StatusForbidden, domain.ErrAccessDenied, 0)
		return
	}

	account, err := h.as.GetAccount(ctx, accountID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err, 1)
		return
	}

	if err := httputil.WriteJSON(w, http.StatusOK, account); err != nil {
		h.log.Error("[GetAccount] json encode", "err", err)
	}
}

type AccountTransactions struct {
	Transactions []domain.Transaction `json:"transactions"`
	Total        int                  `json:"total"`
}

// Get /accounts/:id/transactions?limit=..&offset=...
func (h *Handler) AccountTransactions(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	accountIDStr := r.PathValue("id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	identity, ok := requestctx.IdentityFrom(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, errors.New("user_id не найден в контексте"), 0)
		return
	}
	if identity.UserID != accountID.String() {
		httputil.WriteError(w, http.StatusForbidden, domain.ErrAccessDenied, 0)
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
		httputil.WriteError(w, http.StatusInternalServerError, err, 0)
		return
	}

	dto := AccountTransactions{
		Transactions: transactions,
		Total:        total,
	}

	if err := httputil.WriteJSON(w, http.StatusOK, dto); err != nil {
		h.log.Error("[AccountTransactions] json encode", "err", err)
	}
}
