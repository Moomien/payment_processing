package handlers

import (
	"log/slog"
	"processing/internal/domain"
)

type handler struct {
	ts   domain.TransactionUsecase
	as   domain.AccountsUsecase
	auth domain.AuthUseCase
	log  *slog.Logger
}

func NewHandler(
	ts domain.TransactionUsecase,
	as domain.AccountsUsecase,
	auth domain.AuthUseCase,
	log *slog.Logger,
) *handler {
	return &handler{
		ts:   ts,
		as:   as,
		auth: auth,
		log:  log,
	}
}
