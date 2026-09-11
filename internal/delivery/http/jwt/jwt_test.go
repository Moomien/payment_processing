package jwtLayer

import (
	"errors"
	"processing/internal/infrastructure/config"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerGenerateAndValidate(t *testing.T) {
	manager := NewManager(config.JWTConfig{
		AccessSecret:  "access-secret-for-tests",
		RefreshSecret: "refresh-secret-for-tests",
		AccessTTL:     time.Minute,
		RefreshTTL:    time.Hour,
		Issuer:        "processing-test",
	})

	pair, err := manager.GenerateTokenPair("user-id", "user")
	require.NoError(t, err)

	accessClaims, err := manager.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-id", accessClaims.UserID)
	assert.Equal(t, "user", accessClaims.Role)

	refreshClaims, err := manager.ValidateRefreshToken(pair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, "user-id", refreshClaims.UserID)
	assert.Equal(t, pair.JTI, refreshClaims.ID)
}

func TestManagerRejectsExpiredToken(t *testing.T) {
	manager := NewManager(config.JWTConfig{
		AccessSecret:  "access-secret-for-tests",
		RefreshSecret: "refresh-secret-for-tests",
		AccessTTL:     -time.Minute,
		RefreshTTL:    -time.Minute,
		Issuer:        "processing-test",
	})

	pair, err := manager.GenerateTokenPair("user-id", "user")
	require.NoError(t, err)
	_, err = manager.ValidateRefreshToken(pair.RefreshToken)

	assert.True(t, errors.Is(err, ErrTokenExpired))
}
