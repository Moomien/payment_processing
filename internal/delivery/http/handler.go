package handlers

import "processing/internal/usecase"

type handler struct {
	ts *usecase.TransactionsService
	as *usecase.AccountsService
}

func NewHandler(ts *usecase.TransactionsService, as *usecase.AccountsService) *handler {
	return &handler{ts: ts, as: as}
}
