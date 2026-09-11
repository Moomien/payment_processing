package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"processing/internal/decimal"
	httpapp "processing/internal/delivery/http/app"
	"processing/internal/delivery/http/helpers/httputil"
	"processing/internal/delivery/http/helpers/request"
	"processing/internal/delivery/http/requestctx"
	"processing/internal/domain"

	"github.com/google/uuid"
)

type transferDTO struct {
	Receiver_id uuid.UUID `json:"receiver_id"`
	Amount      string    `json:"amount"`
}

type Handler struct {
	ts  domain.TransactionUsecase
	log *slog.Logger
}

func New(app *httpapp.App) *Handler {
	return &Handler{
		ts:  app.TransactionUsecase,
		log: app.Log,
	}
}

func NewHandler(ts domain.TransactionUsecase, as domain.AccountsUsecase, auth domain.AuthUseCase, log *slog.Logger) *Handler {
	return &Handler{
		ts:  ts,
		log: log,
	}
}

// Transfer хэндлер для оплаты
// POST /transactions
func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	identity, ok := requestctx.IdentityFrom(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, errors.New("user_id не найден в контексте"), 0)
		return
	}

	senderID, err := uuid.Parse(identity.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	key := r.Header.Get("Idempotency-Key")
	if err := domain.ValidateIdempotencyKey(key); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 1)
		return
	}

	var dto transferDTO
	if err := httputil.DecodeJSON(w, r, &dto); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	amount, err := decimal.NewFromString(dto.Amount)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	transactionID, err := h.ts.Transfer(ctx, senderID, dto.Receiver_id, key, amount)
	if err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) || errors.Is(err, domain.ErrIdempotencyInProgress) {
			httputil.WriteError(w, http.StatusConflict, err, 1)
			return
		}
		if errors.Is(err, domain.ErrSameAccount) || errors.Is(err, domain.ErrInvalidAmount) || errors.Is(err, domain.ErrInsufficientFunds) || errors.Is(err, domain.ErrReceiverAccountNotFound) {
			httputil.WriteError(w, http.StatusBadRequest, err, 1)
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, err, 1)
		return
	}

	if err := httputil.WriteJSON(w, http.StatusCreated, map[string]string{"transaction_id": transactionID}); err != nil {
		h.log.Error("[transfer] json encode", "err", err)
	}
}

type transactionDTO struct {
	UserID         uuid.UUID `json:"user_id"`
	IdempotencyKEY string    `json:"Idempotency-Key"`
}

// получение транзакции по id
// GET /transactions/:id
func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	id, err := uuid.Parse(r.PathValue("id"))
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 1)
		return
	}

	ctx := r.Context()
	identity, ok := requestctx.IdentityFrom(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, errors.New("user_id не найден в контексте"), 0)
		return
	}

	userID, err := uuid.Parse(identity.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	key := r.Header.Get("Idempotency-Key")
	transaction, err := h.ts.GetTransaction(ctx, id, userID, key)
	if err != nil {
		if errors.Is(err, domain.ErrAccessDenied) {
			httputil.WriteError(w, http.StatusNotFound, err, 0)
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, err, 1)
		return
	}

	if err := httputil.WriteJSON(w, http.StatusOK, transaction); err != nil {
		h.log.Error("[GetTransaction] json encode", "err", err)
	}
}

// выводит транзакции по фильтрам
// Примеры запросов:
// GET /transactions?sender_id=uuid&from=2024-01-01&limit=20&offset=0
// GET /transactions?receiver_id=uuid&min_amount=1000
func (h *Handler) TransactionFilter(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/json")

	ctx := r.Context()
	identity, ok := requestctx.IdentityFrom(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, errors.New("user_id не найден в контексте"), 0)
		return
	}

	userID, err := uuid.Parse(identity.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	key := r.Header.Get("Idempotency-Key")
	query := r.URL.Query()
	filter, err := request.NewTransactionFilter(query)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 1)
		return
	}

	transactions, err := h.ts.GetTransactionFilter(ctx, filter, userID, key)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err, 1)
		return
	}

	if err := httputil.WriteJSON(w, http.StatusOK, transactions); err != nil {
		h.log.Error("[TransactionFilter] json encode", "err", err)
	}
}
