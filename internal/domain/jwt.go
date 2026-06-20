package domain

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AuthUseCase interface {
	Register(ctx context.Context, email, password, name string, ip string) (*Account, error)
	Login(ctx context.Context, email, password string, ip string) (*TokenPair, error)
	Refresh(ctx context.Context, refreshToken string, ip string) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string, ip string) error
	LogoutAll(ctx context.Context, userID uuid.UUID) error
}

// RefreshSession представляет сессию refresh токена в БД
type RefreshSession struct {
	UserID    uuid.UUID
	Revoked   bool
	ExpiresAt time.Time
}

// TokenPair пара токенов для клиента
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int64     `json:"expires_in"`
	JTI          string    `json:"-"`
	ExpiresAt    time.Time `json:"-"`
}

type AccessClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}
