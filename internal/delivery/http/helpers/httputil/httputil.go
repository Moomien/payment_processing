package httputil

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"processing/internal/domain"
	"strings"
)

const maxJSONBodyBytes = 64 << 10

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)

	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("Тело запрос должно содержать ровно один json объект!")
		}
		return err
	}
	return nil
}

func status(id int) string {
	m := map[int]string{
		200: "OK",
		400: "Bad Request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		409: "Conflict",
		422: "Unprocessable Entity",
		429: "Too Many Requests",
		500: "Internal Server Error",
	}
	value, _ := m[id]
	return value
}

// writeError пишет ошибку клиенту.
// flag:  1 - полная ошибка, 0 - только часть
func WriteError(
	w http.ResponseWriter,
	code int,
	err error,
	flag int,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if flag == 1 {
		json.NewEncoder(w).Encode(map[string]string{
			"error":   status(code),
			"message": err.Error(),
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"error": status(code)})
}

func WriteAuthError(w http.ResponseWriter, err error) {
	switch {
	//rate limit
	case errors.Is(err, domain.ErrRateLimited):
		WriteError(w, http.StatusTooManyRequests, err, 0)
	//аккаунт уже есть
	case errors.Is(err, domain.ErrAccountAlreadyExist):
		WriteError(w, http.StatusConflict, err, 0)
	//невалидные данные
	case errors.Is(err, domain.ErrInvalidEmail),
		errors.Is(err, domain.ErrInvalidPassword),
		errors.Is(err, domain.ErrInvalidName):
		WriteError(w, http.StatusUnprocessableEntity, err, 0)
	//невалидные креды
	case errors.Is(err, domain.ErrInvalidCredentials),
		errors.Is(err, domain.ErrInvalidRefreshToken),
		errors.Is(err, domain.ErrRefreshTokenExpired),
		errors.Is(err, domain.ErrRefreshTokenRevoked),
		errors.Is(err, domain.ErrRefreshTokenReuse):
		WriteError(w, http.StatusUnauthorized, err, 0)
	default:
		WriteError(w, http.StatusInternalServerError, err, 0)
	}
}

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.Trim(strings.TrimSpace(r.RemoteAddr), "[]")
}

func WriteJSON(w http.ResponseWriter, code int, v any) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := buf.WriteTo(w)
	return err
}

func SetAuthCookie(
	w http.ResponseWriter,
	path,
	name,
	token string,
	maxage int,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     path,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxage,
	})
}
