package app

import (
	"log/slog"
	"processing/internal/domain"
)

type App struct {
	TransactionUsecase domain.TransactionUsecase
	AccountsUsecase    domain.AccountsUsecase
	AuthUseCase        domain.AuthUseCase
	Log                *slog.Logger
}

func NewApp(
	ts domain.TransactionUsecase,
	as domain.AccountsUsecase,
	auth domain.AuthUseCase,
	log *slog.Logger,
) *App {
	return &App{
		TransactionUsecase: ts,
		AccountsUsecase:    as,
		AuthUseCase:        auth,
		Log:                log,
	}
}
