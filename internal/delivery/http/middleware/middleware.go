package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	jwtLayer "processing/internal/delivery/http/jwt"
	"strings"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxRole
)

type Auth struct {
	jwt *jwtLayer.Manager
}

func NewAuth(jwt *jwtLayer.Manager) Auth {
	return Auth{jwt: jwt}
}

func (a Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			unauthorized(w, "требуется авторизация")
			return
		}

		claims, err := a.jwt.ValidateAccessToken(token)
		if err != nil {
			if errors.Is(err, jwtLayer.ErrTokenExpired) {
				unauthorized(w, "токен истёк")
				return
			}
			unauthorized(w, "невалидный токен")
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxRole, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	scheme, token, found := strings.Cut(header, " ")
	if !found {
		return "", errors.New("заголовок Authorization отсутствует или кэш пуст")
	}

	if !strings.EqualFold(scheme, "Bearer") {
		return "", errors.New("ожидается схема Bearer")
	}

	if token == "" {
		return "", errors.New("токен пуст")
	}
	return token, nil
}

func unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
