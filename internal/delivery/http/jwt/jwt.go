package jwtLayer

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AccessClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int64     `json:"expires_in"`
	JTI          string    `json:"-"`
	ExpiresAt    time.Time `json:"-"`
}

const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour
)

func GenerateTokenPair(userID string, role string) (*TokenPair, error) {
	accessSecretKey := os.Getenv("accessSecretKey")
	refreshSecretKey := os.Getenv("refreshSecretKey")

	if accessSecretKey == "" || refreshSecretKey == "" {
		return nil, errors.New("секретные ключи не установлены в переменных окружения")
	}

	now := time.Now()
	accessExpiresAt := now.Add(AccessTokenDuration)
	refreshExpiresAt := now.Add(RefreshTokenDuration)

	accessClaims := AccessClaims{
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

	refreshClaims := RefreshClaims{
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

	return &TokenPair{
		AccessToken:  accessSigned,
		RefreshToken: refreshSigned,
		ExpiresIn:    int64(AccessTokenDuration.Seconds()),
		JTI:          jti.String(),
		ExpiresAt:    refreshExpiresAt,
	}, nil
}

func ValidateAccessToken(tokenString string) (*AccessClaims, error) {
	accessSecretKey := os.Getenv("accessSecretKey")

	token, err := jwt.ParseWithClaims(
		tokenString, &AccessClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("неверный алгоритм: %v", t.Header["alg"])
			}
			return accessSecretKey, nil
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

	claims, ok := token.Claims.(*AccessClaims)
	if !ok {
		return nil, errors.New("невалидные claims")
	}

	return claims, nil
}

func ValidateRefreshToken(tokenString string) (*RefreshClaims, error) {
	refreshSecretKey := os.Getenv("refreshSecretKey")

	token, err := jwt.ParseWithClaims(
		tokenString,
		&RefreshClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("невалидный алгоритм: %v", t.Header["alg"])
			}
			return refreshSecretKey, nil
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

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok {
		return nil, errors.New("невалидные claims")
	}

	return claims, nil
}
