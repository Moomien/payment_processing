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
		writeError(w, http.StatusBadRequest, err, 0)
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

	token, err := h.auth.Login(ctx, dto.Email, dto.Password, ip)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
		return
	}

	setAuthCookie(w, "/api", "access_token", token.AccessToken, 900)
	setAuthCookie(w, "/auth/refresh", "refresh_token", token.RefreshToken, 604800)
	if err := writeJSON(w, http.StatusCreated, map[string]interface{}{
		"account": account,
		"tokens":  token,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err, 1) //
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

	setAuthCookie(w, "/api", "access_token", token.AccessToken, 900)
	setAuthCookie(w, "/auth/refresh", "refresh_token", token.RefreshToken, 604800)
	if err := writeJSON(w, http.StatusOK, token); err != nil {
		writeError(w, http.StatusInternalServerError, err, 1) //
	}
}

func (h *handler) Refresh(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	cookie, err := r.Cookie("refresh_token")
	var refreshToken string
	if err == nil {
		refreshToken = cookie.Value
	} else {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}
	ip := r.RemoteAddr
	if refreshToken == "" {
		writeError(w, http.StatusBadRequest, errors.New("refresh token отсутствует"), 1)
		return
	}

	token, err := h.auth.Refresh(ctx, refreshToken, ip)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
		return
	}

	setAuthCookie(w, "/api", "access_token", token.AccessToken, 900)
	setAuthCookie(w, "/auth/refresh", "refresh_token", token.RefreshToken, 604800)
	if err := writeJSON(w, http.StatusOK, token); err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
	}
}

func (h *handler) Logout(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	var refreshToken string
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		refreshToken = cookie.Value
	} else {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("refresh token отсутствует"), 0)
			return
		}
		refreshToken = req.RefreshToken
	}
	ip := r.RemoteAddr
	if refreshToken == "" {
		writeError(w, http.StatusBadRequest, errors.New("refresh token отсутствует"), 0)
		return
	}

	if err := h.auth.Logout(ctx, refreshToken, ip); err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
	}
	setAuthCookie(w, "/api", "access_token", "", -1)
	setAuthCookie(w, "/auth/refresh", "refresh_token", "", -1)
	if err := writeJSON(w, http.StatusOK, map[string]string{"message": "success"}); err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
	}
}

func (h *handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	ctxUserID, ok := r.Context().Value("user_id").(string)
	if !ok {
		writeError(w, http.StatusBadRequest, errors.New("поле user_id должно быть string"), 1)
		return
	}

	userID, err := uuid.Parse(ctxUserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err, 0)
		return
	}

	if err := h.auth.LogoutAll(ctx, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
	}
	setAuthCookie(w, "/api", "access_token", "", -1)
	setAuthCookie(w, "/auth/refresh", "refresh_token", "", -1)
	if err := writeJSON(w, http.StatusOK, map[string]string{"message": "success"}); err != nil {
		writeError(w, http.StatusInternalServerError, err, 0)
	}
}
