package middleware

import (
	"net/http"
	"net/http/httptest"
	jwtLayer "processing/internal/delivery/http/jwt"
	"processing/internal/delivery/http/requestctx"
	"processing/internal/infrastructure/config"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddlewarePassesIdentityToHandler(t *testing.T) {
	const (
		userID = "123e4567-e89b-12d3-a456-426614174000"
		role   = "user"
	)

	manager := jwtLayer.NewManager(config.JWTConfig{
		AccessSecret:  "test-access-secret",
		RefreshSecret: "test-refresh-secret",
		AccessTTL:     time.Minute,
		RefreshTTL:    time.Hour,
		Issuer:        "middleware-test",
	})
	tokens, err := manager.GenerateTokenPair(userID, role)
	require.NoError(t, err)

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		identity, ok := requestctx.IdentityFrom(r.Context())
		require.True(t, ok)
		assert.Equal(t, userID, identity.UserID)
		assert.Equal(t, role, identity.Role)
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()

	NewAuth(manager).Middleware(next).ServeHTTP(rec, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
