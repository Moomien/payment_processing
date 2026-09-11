package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPostgresDoesNotRequireServerConfig(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_DB", "processing")
	t.Setenv("POSTGRES_USER", "admin")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_SSLMODE", SSLModeDisable)
	t.Setenv("POSTGRES_MAX_CONNS", "10")
	t.Setenv("ACCESS_TOKEN_SECRET", "")
	t.Setenv("REFRESH_TOKEN_SECRET", "")

	cfg, err := LoadPostgres()

	require.NoError(t, err)
	assert.Equal(t, "postgres", cfg.HOST)
	assert.Equal(t, SSLModeDisable, cfg.SSLMODE)
}

func TestLoadPostgresRejectsInvalidSSLMode(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_DB", "processing")
	t.Setenv("POSTGRES_USER", "admin")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_SSLMODE", "invalid")
	t.Setenv("POSTGRES_MAX_CONNS", "10")

	_, err := LoadPostgres()

	require.Error(t, err)
	assert.ErrorContains(t, err, "POSTGRES_SSLMODE")
}
