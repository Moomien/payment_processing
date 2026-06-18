package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"processing/internal/domain"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func newTransactionFilter(query url.Values) (*domain.TransactionFilter, error) {
	senderID, err := parseUUID(query, "sender_id")
	if err != nil {
		return nil, err
	}
	receiverID, err := parseUUID(query, "receiver_id")
	if err != nil {
		return nil, err
	}

	minAmount := query.Get("min_amount")
	maxAmount := query.Get("max_amount")

	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil {
		return nil, fmt.Errorf("невалидный limit: %w", err)
	}
	offset, err := strconv.Atoi(query.Get("offset"))
	if err != nil {
		return nil, fmt.Errorf("невалидный offset: %w", err)
	}

	from, err := parseTime(query, "from")
	if err != nil {
		return nil, err
	}

	to, err := parseTime(query, "to")
	if err != nil {
		return nil, err
	}

	return &domain.TransactionFilter{
		SenderID:   senderID,
		ReceiverID: receiverID,
		MinAmount:  minAmount,
		MaxAmount:  maxAmount,
		From:       from,
		To:         to,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

func parseUUID(query url.Values, key string) (uuid.UUID, error) {
	val := query.Get(key)
	if val == "" {
		return uuid.UUID{}, nil
	}

	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("невилдный %s", val)
	}
	return id, nil
}

func parseTime(query url.Values, key string) (time.Time, error) {
	val := query.Get(key)
	if val == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse("2006-01-02", val)
	if err != nil {
		return time.Time{}, fmt.Errorf("невалидная дата %s: %w", key, err)
	}

	return t, nil
}

func status(id int) string {
	m := map[int]string{
		200: "OK",
		400: "StatusBadRequest",
		401: "Unauthorized",
		403: "Forbidden",
		404: "StatusNotFound",
		429: "too many requests",
		500: "internal server error",
	}
	value, _ := m[id]
	return value
}

// writeError пишет ошибку клиенту.
// flag:  1 - полная ошибка, любой другой - только часть
func writeError(w http.ResponseWriter, code int, err error, flag int) {
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

func writeJSON(w http.ResponseWriter, code int, v any) error {
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

func setAuthCookie(w http.ResponseWriter, path, name, token string, maxage int) {
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

func validateLogin(w http.ResponseWriter, data *AuthDTO) bool {
	data.Email = strings.TrimSpace(data.Email)

	if data.Email == "" {
		http.Error(w, "поле с почтой не может быть пустым", http.StatusUnprocessableEntity)
		return false
	}

	if data.Password == "" {
		http.Error(w, "поле с паролем не может быть пустым", http.StatusUnprocessableEntity)
		return false
	}

	regmail := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	reg := regexp.MustCompile(regmail)
	if !reg.MatchString(data.Email) {
		http.Error(
			w,
			"Пожалуйста, введите корректный адрес электронной почты (например, example@mail.com)",
			http.StatusUnprocessableEntity,
		)
		return false
	}

	if len(data.Password) < 8 {
		http.Error(w, "длина пароля не может быть меньше 8 символов", http.StatusUnprocessableEntity)
		return false
	}

	return true
}

func validateRegister(w http.ResponseWriter, data *AuthDTO) bool {
	data.Email = strings.TrimSpace(data.Email)
	data.Name = strings.TrimSpace(data.Name)

	if data.Name == "" {
		http.Error(w, "поле с именем не может быть пустым", http.StatusUnprocessableEntity)
		return false
	}

	if data.Email == "" {
		http.Error(w, "поле с почтой не может быть пустым", http.StatusUnprocessableEntity)
		return false
	}

	if data.Password == "" {
		http.Error(w, "поле с паролем не может быть пустым", http.StatusUnprocessableEntity)
		return false
	}

	if len(data.Name) < 3 {
		http.Error(w, "имя не может быть меньше 3 букв", http.StatusUnprocessableEntity)
		return false
	}

	regmail := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	reg := regexp.MustCompile(regmail)
	if !reg.MatchString(data.Email) {
		http.Error(
			w,
			"Пожалуйста, введите корректный адрес электронной почты (например, example@mail.com)",
			http.StatusUnprocessableEntity,
		)
		return false
	}

	if len(data.Password) < 8 {
		http.Error(w, "длина пароля не может быть меньше 8 символов", http.StatusUnprocessableEntity)
		return false
	}

	return true
}
