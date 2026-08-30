package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	Health(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("unexpected content type: got %q", contentType)
	}

	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["status"] != "ok" {
		t.Fatalf("unexpected health status: got %q", response["status"])
	}
}

func TestReadiness(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		rec := httptest.NewRecorder()
		Readiness(func(context.Context) error { return nil })(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("dependency unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		Readiness(func(context.Context) error { return errors.New("down") })(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}
