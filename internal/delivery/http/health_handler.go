package handlers

import (
	"context"
	"net/http"
	"time"
)

type HealthCheck func(context.Context) error

func Health(w http.ResponseWriter, _ *http.Request) {
	if err := writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func Readiness(checks ...HealthCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		for _, check := range checks {
			if err := check(ctx); err != nil {
				_ = writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
				return
			}
		}
		_ = writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
