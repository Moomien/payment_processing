package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	httpapp "processing/internal/delivery/http/app"
	"processing/internal/delivery/http/helpers/httputil"
	"processing/internal/delivery/http/requestctx"
	"processing/internal/domain"

	"github.com/google/uuid"
)

type AuthDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"username"`
}

type Handler struct {
	auth domain.AuthUseCase
	log  *slog.Logger
}

type handler = Handler

func New(app *httpapp.App) *Handler {
	return &Handler{
		auth: app.AuthUseCase,
		log:  app.Log,
	}
}

func NewHandler(auth domain.AuthUseCase, log *slog.Logger) *Handler {
	return &Handler{
		auth: auth,
		log:  log,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	ip := httputil.ClientIP(r)

	var dto AuthDTO
	if err := httputil.DecodeJSON(w, r, &dto); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	account, err := h.auth.Register(ctx, dto.Email, dto.Password, dto.Name, ip)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	token, err := h.auth.Login(ctx, dto.Email, dto.Password, ip)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	httputil.SetAuthCookie(w, "/api", "access_token", token.AccessToken, 900)
	httputil.SetAuthCookie(w, "/auth/refresh", "refresh_token", token.RefreshToken, 604800)
	if err := httputil.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"account": account,
		"tokens":  token,
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err, 1)
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	ip := httputil.ClientIP(r)

	var dto AuthDTO
	if err := httputil.DecodeJSON(w, r, &dto); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	token, err := h.auth.Login(ctx, dto.Email, dto.Password, ip)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	httputil.SetAuthCookie(w, "/api", "access_token", token.AccessToken, 900)
	httputil.SetAuthCookie(w, "/auth/refresh", "refresh_token", token.RefreshToken, 604800)
	if err := httputil.WriteJSON(w, http.StatusOK, token); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err, 1)
	}
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
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
		if err := httputil.DecodeJSON(w, r, &req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, errors.New("refresh token отсутствует"), 1)
			return
		}
		refreshToken = req.RefreshToken
	}
	ip := httputil.ClientIP(r)
	if refreshToken == "" {
		httputil.WriteError(w, http.StatusBadRequest, errors.New("refresh token отсутствует"), 1)
		return
	}

	token, err := h.auth.Refresh(ctx, refreshToken, ip)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	httputil.SetAuthCookie(w, "/api", "access_token", token.AccessToken, 900)
	httputil.SetAuthCookie(w, "/auth/refresh", "refresh_token", token.RefreshToken, 604800)
	if err := httputil.WriteJSON(w, http.StatusOK, token); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err, 0)
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
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
		if err := httputil.DecodeJSON(w, r, &req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, errors.New("refresh token отсутствует"), 0)
			return
		}
		refreshToken = req.RefreshToken
	}
	ip := httputil.ClientIP(r)
	if refreshToken == "" {
		httputil.WriteError(w, http.StatusBadRequest, errors.New("refresh token отсутствует"), 0)
		return
	}

	if err := h.auth.Logout(ctx, refreshToken, ip); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	httputil.SetAuthCookie(w, "/api", "access_token", "", -1)
	httputil.SetAuthCookie(w, "/auth/refresh", "refresh_token", "", -1)
	if err := httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "success"}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err, 0)
	}
}

func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	identity, ok := requestctx.IdentityFrom(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusBadRequest, errors.New("поле user_id должно быть string"), 1)
		return
	}

	userID, err := uuid.Parse(identity.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err, 0)
		return
	}

	if err := h.auth.LogoutAll(ctx, userID); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	httputil.SetAuthCookie(w, "/api", "access_token", "", -1)
	httputil.SetAuthCookie(w, "/auth/refresh", "refresh_token", "", -1)
	if err := httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "success"}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, err, 0)
	}
}
