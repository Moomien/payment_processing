package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type AuthDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"username"`
}

func (h *handler) Register(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	ip := r.RemoteAddr

	var dto AuthDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, 500, err, 0)
		return
	}

	if !validateRegister(w, &dto) {
		return
	}

	account, err := h.auth.Register(ctx, dto.Email, dto.Password, dto.Name, ip)
	if err != nil {
		writeError(w, 500, err, 0)
		return
	}

	if err := writeJSON(w, http.StatusOK, account); err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
	}
}

func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	ip := r.RemoteAddr

	var dto AuthDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, 400, err, 0)
		return
	}

	if !validateLogin(w, &dto) {
		return
	}

	token, err := h.auth.Login(ctx, dto.Email, dto.Password, ip)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, 1)
		return
	}

	setAuthCookie(w, "/", "access_token", token.AccessToken, 900)
	setAuthCookie(w, "/", "refresh_token", token.RefreshToken, 604800)
}

func (h *handler) Refresh(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	refreshToken := r.Header.Get("refresh_token")
	ip := r.RemoteAddr

	tokenpair, err := h.auth.Refresh(ctx, refreshToken, ip)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
		return
	}

	if err := writeJSON(w, http.StatusOK, tokenpair); err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
	}
}

func (h *handler) Logout(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	ctxUserID, ok := r.Context().Value("user_id").(string)
	if !ok {
		writeError(w, http.StatusBadRequest, errors.New("поле user_id должно быть string"), 0)
		return
	}
	userID, err := uuid.Parse(ctxUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, 1)
		return
	}
	if err := h.auth.LogoutAll(ctx, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
		return
	}
}
