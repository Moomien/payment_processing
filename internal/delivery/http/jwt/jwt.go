package jwtLayer

import (
	"errors"
	"fmt"
	"os"
	"processing/internal/domain"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour
)

func GenerateTokenPair(userID string, role string) (*domain.TokenPair, error) {
	accessSecretKey := os.Getenv("accessSecretKey")
	refreshSecretKey := os.Getenv("refreshSecretKey")

	if accessSecretKey == "" || refreshSecretKey == "" {
		return nil, errors.New("секретные ключи не установлены в переменных окружения")
	}

	now := time.Now()
	accessExpiresAt := now.Add(AccessTokenDuration)
	refreshExpiresAt := now.Add(RefreshTokenDuration)

	accessClaims := domain.AccessClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "my-app",
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessSigned, err := accessToken.SignedString([]byte(accessSecretKey))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания access token: %w", err)
	}

	jti, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации JTI: %w", err)
	}

	refreshClaims := domain.RefreshClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti.String(),
			ExpiresAt: jwt.NewNumericDate(refreshExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "my-app",
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshSigned, err := refreshToken.SignedString([]byte(refreshSecretKey))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessSigned,
		RefreshToken: refreshSigned,
		ExpiresIn:    int64(AccessTokenDuration.Seconds()),
		JTI:          jti.String(),
		ExpiresAt:    refreshExpiresAt,
	}, nil
}

func ValidateAccessToken(tokenString string) (*domain.AccessClaims, error) {
	accessSecretKey := os.Getenv("accessSecretKey")

	token, err := jwt.ParseWithClaims(
		tokenString, &domain.AccessClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("неверный алгоритм: %v", t.Header["alg"])
			}
			return []byte(accessSecretKey), nil
		},
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("истекший токен")
		} else if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, errors.New("подпись неверна")
		}
		return nil, fmt.Errorf("парсинг jwt токена: %w", err)
	}

	claims, ok := token.Claims.(*domain.AccessClaims)
	if !ok {
		return nil, errors.New("невалидные claims")
	}

	return claims, nil
}

func ValidateRefreshToken(tokenString string) (*domain.RefreshClaims, error) {
	refreshSecretKey := os.Getenv("refreshSecretKey")

	token, err := jwt.ParseWithClaims(
		tokenString,
		&domain.RefreshClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("невалидный алгоритм: %v", t.Header["alg"])
			}
			return []byte(refreshSecretKey), nil
		},
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("истекший токен")
		} else if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, errors.New("подпись неверна")
		}
		return nil, fmt.Errorf("парсинг jwt токена: %w", err)
	}

	claims, ok := token.Claims.(*domain.RefreshClaims)
	if !ok {
		return nil, errors.New("невалидные claims")
	}

	return claims, nil
}
