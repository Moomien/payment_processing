package router

import (
	"net/http"

	httpapp "processing/internal/delivery/http/app"
	accountshandlers "processing/internal/delivery/http/handlers/accounts"
	authhandlers "processing/internal/delivery/http/handlers/auth"
	httphealth "processing/internal/delivery/http/handlers/health"
	transactionhandlers "processing/internal/delivery/http/handlers/transactions"
	"processing/internal/delivery/http/middleware"
)

func New(app *httpapp.App, auth middleware.Auth, checks ...httphealth.HealthCheck) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", httphealth.Health)
	mux.HandleFunc("GET /health/live", httphealth.Health)
	mux.Handle("GET /health/ready", httphealth.Readiness(checks...))

	authHandler := authhandlers.New(app)
	accountsHandler := accountshandlers.New(app)
	transactionsHandler := transactionhandlers.New(app)

	mux.HandleFunc("POST /auth/register", authHandler.Register)
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/refresh", authHandler.Refresh)
	mux.Handle("POST /auth/logout", auth.Middleware(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("POST /auth/logout-all", auth.Middleware(http.HandlerFunc(authHandler.LogoutAll)))
	mux.Handle("GET /accounts/{id}", auth.Middleware(http.HandlerFunc(accountsHandler.GetAccount)))
	mux.Handle("GET /accounts/{id}/transactions", auth.Middleware(http.HandlerFunc(accountsHandler.AccountTransactions)))
	mux.Handle("POST /transactions", auth.Middleware(http.HandlerFunc(transactionsHandler.Transfer)))
	mux.Handle("GET /transactions/{id}", auth.Middleware(http.HandlerFunc(transactionsHandler.GetTransaction)))
	mux.Handle("GET /transactions", auth.Middleware(http.HandlerFunc(transactionsHandler.TransactionFilter)))

	return mux
}
