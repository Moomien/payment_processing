package handlers

import (
	"log/slog"
	"processing/internal/usecase"
)

type handler struct {
	ts  *usecase.TransactionsService
	as  *usecase.AccountsService
	log *slog.Logger
}

func NewHandler(ts *usecase.TransactionsService, as *usecase.AccountsService, log *slog.Logger) *handler {
	return &handler{
		ts:  ts,
		as:  as,
		log: log,
	}
}
