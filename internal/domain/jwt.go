package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// RefreshSession представляет сессию refresh токена в БД
type RefreshSession struct {
	UserID    uuid.UUID
	Revoked   bool
	ExpiresAt time.Time
}

// TokenPair пара токенов для клиента
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// TokenPairInternal внутренняя структура для usecase слоя
type TokenPairInternal struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	JTI          string
	ExpiresAt    time.Time
}

type AccessClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID string `json:"user_id"`
	JTI    string `json:"jti"`
	jwt.RegisteredClaims
}
