package jwtLayer

import (
	"errors"
	"fmt"
	"processing/internal/domain"
	"processing/internal/infrastructure/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrTokenExpired = errors.New("токен истёк")
	ErrTokenInvalid = errors.New("токен невалиден")
)

type Manager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	issuer        string
}

func NewManager(cfg config.JWTConfig) *Manager {
	return &Manager{
		accessSecret:  []byte(cfg.AccessSecret),
		refreshSecret: []byte(cfg.RefreshSecret),
		accessTTL:     cfg.AccessTTL,
		refreshTTL:    cfg.RefreshTTL,
		issuer:        cfg.Issuer,
	}
}

func (m *Manager) GenerateTokenPair(userID string, role string) (*domain.TokenPair, error) {
	now := time.Now()
	accessExpiresAt := now.Add(m.accessTTL)
	refreshExpiresAt := now.Add(m.refreshTTL)

	accessClaims := domain.AccessClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    m.issuer,
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessSigned, err := accessToken.SignedString(m.accessSecret)
	if err != nil {
		return nil, fmt.Errorf("подпись access token: %w", err)
	}

	jti, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("генерации JTI: %w", err)
	}

	refreshClaims := domain.RefreshClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti.String(),
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(refreshExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    m.issuer,
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshSigned, err := refreshToken.SignedString(m.refreshSecret)
	if err != nil {
		return nil, fmt.Errorf("подпись refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessSigned,
		RefreshToken: refreshSigned,
		ExpiresIn:    int64(m.accessTTL.Seconds()),
		JTI:          jti.String(),
		ExpiresAt:    refreshExpiresAt,
	}, nil
}

func (m *Manager) ValidateAccessToken(tokenString string) (*domain.AccessClaims, error) {
	claims := &domain.AccessClaims{}
	if err := m.parse(tokenString, claims, m.accessSecret); err != nil {
		return nil, err
	}

	return claims, nil
}

func (m *Manager) ValidateRefreshToken(tokenString string) (*domain.RefreshClaims, error) {
	claims := &domain.RefreshClaims{}
	if err := m.parse(tokenString, claims, m.refreshSecret); err != nil {
		return nil, err
	}
	if claims.ID == "" {
		return nil, fmt.Errorf("%w: отсутствует jti", ErrTokenInvalid)
	}
	return claims, nil
}

func (m *Manager) parse(tokenString string, claims jwt.Claims, secret []byte) error {
	token, err := jwt.ParseWithClaims(
		tokenString, claims,
		func(t *jwt.Token) (interface{}, error) {
			return secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
	)
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return ErrTokenExpired
	case err != nil:
		return fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	case !token.Valid:
		return ErrTokenInvalid
	}
	return nil
}
