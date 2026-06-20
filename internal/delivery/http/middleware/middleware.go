package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	jwtLayer "processing/internal/delivery/http/jwt"
	"processing/internal/domain"
	"strings"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := validateToken(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "role", claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func validateToken(r *http.Request) (*domain.AccessClaims, error) {
	var token string
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return nil, errors.New("неверный формат authorization header")
		}
		token = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			return nil, errors.New("токен отсутствует")
		}
		token = cookie.Value
	}

	if token == "" {
		return nil, errors.New("токен пустой")
	}

	claims, err := jwtLayer.ValidateAccessToken(token)
	if err != nil {
		return nil, fmt.Errorf("невалидный токен: %w", err)
	}

	if claims.UserID == "" {
		return nil, errors.New("user_id отсутствует в токене")
	}

	return claims, nil
}
