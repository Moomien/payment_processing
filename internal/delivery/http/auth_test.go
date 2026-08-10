package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"processing/internal/decimal"
	"processing/internal/delivery/http/mocks"
	"processing/internal/domain"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRegisterHandler(t *testing.T) {
	testUserID := uuid.New()
	testBalance, _ := decimal.NewFromString("0")
	testAccount := &domain.Account{
		ID:      testUserID,
		Name:    "TestUser",
		Email:   "test@example.com",
		Balance: testBalance,
		Role:    "user",
	}
	testTokenPair := &domain.TokenPair{
		AccessToken:  "test_access_token",
		RefreshToken: "test_refresh_token",
		ExpiresIn:    900,
	}

	tests := []struct {
		name               string
		requestBody        interface{}
		setupMock          func(*mocks.AuthUseCase)
		expectedStatusCode int
		checkResponse      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "успешная регистрация",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "TestUser",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "test@example.com", "password123", "TestUser", mock.Anything).
					Return(testAccount, nil)
				authMock.On("Login", mock.Anything, "test@example.com", "password123", mock.Anything).
					Return(testTokenPair, nil)
			},
			expectedStatusCode: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "account")
				assert.Contains(t, response, "tokens")

				cookies := rec.Result().Cookies()
				assert.Len(t, cookies, 2)

				var accessCookie, refreshCookie *http.Cookie
				for _, cookie := range cookies {
					if cookie.Name == "access_token" {
						accessCookie = cookie
					}
					if cookie.Name == "refresh_token" {
						refreshCookie = cookie
					}
				}

				assert.NotNil(t, accessCookie)
				assert.Equal(t, "test_access_token", accessCookie.Value)
				assert.Equal(t, "/api", accessCookie.Path)
				assert.Equal(t, 900, accessCookie.MaxAge)

				assert.NotNil(t, refreshCookie)
				assert.Equal(t, "test_refresh_token", refreshCookie.Value)
				assert.Equal(t, "/auth/refresh", refreshCookie.Path)
				assert.Equal(t, 604800, refreshCookie.MaxAge)
			},
		},
		{
			name:               "невалидный JSON",
			requestBody:        "invalid json",
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "пустое имя",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "test@example.com", "password123", "", mock.Anything).
					Return(nil, domain.ErrInvalidName)
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "имя короче 3 символов",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "ab",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "test@example.com", "password123", "ab", mock.Anything).
					Return(nil, domain.ErrInvalidName)
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "пустой email",
			requestBody: AuthDTO{
				Email:    "",
				Password: "password123",
				Name:     "TestUser",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "", "password123", "TestUser", mock.Anything).
					Return(nil, domain.ErrInvalidEmail)
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "невалидный формат email",
			requestBody: AuthDTO{
				Email:    "invalid-email",
				Password: "password123",
				Name:     "TestUser",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "invalid-email", "password123", "TestUser", mock.Anything).
					Return(nil, domain.ErrInvalidEmail)
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "пустой пароль",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "",
				Name:     "TestUser",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "test@example.com", "", "TestUser", mock.Anything).
					Return(nil, domain.ErrInvalidPassword)
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "пароль короче 8 символов",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "pass123",
				Name:     "TestUser",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "test@example.com", "pass123", "TestUser", mock.Anything).
					Return(nil, domain.ErrInvalidPassword)
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "ошибка при регистрации",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "TestUser",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "test@example.com", "password123", "TestUser", mock.Anything).
					Return(nil, errors.New("database error"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "ошибка при автоматическом логине после регистрации",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "TestUser",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "test@example.com", "password123", "TestUser", mock.Anything).
					Return(testAccount, nil)
				authMock.On("Login", mock.Anything, "test@example.com", "password123", mock.Anything).
					Return(nil, errors.New("login failed"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "email с пробелами обрезается",
			requestBody: AuthDTO{
				Email:    "  test@example.com  ",
				Password: "password123",
				Name:     "  TestUser  ",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Register", mock.Anything, "  test@example.com  ", "password123", "  TestUser  ", mock.Anything).
					Return(testAccount, nil)
				authMock.On("Login", mock.Anything, "  test@example.com  ", "password123", mock.Anything).
					Return(testTokenPair, nil)
			},
			expectedStatusCode: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "account")
				assert.Contains(t, response, "tokens")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMock := mocks.NewAuthUseCase(t)
			tt.setupMock(authMock)

			handler := &handler{
				auth: authMock,
			}

			var body []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				assert.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
			req.RemoteAddr = "127.0.0.1:1234"
			req = req.WithContext(context.Background())
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			assert.Equal(t, tt.expectedStatusCode, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestLoginHandler(t *testing.T) {
	testTokenPair := &domain.TokenPair{
		AccessToken:  "test_access_token",
		RefreshToken: "test_refresh_token",
		ExpiresIn:    900,
	}

	tests := []struct {
		name               string
		requestBody        interface{}
		setupMock          func(*mocks.AuthUseCase)
		expectedStatusCode int
		checkResponse      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "успешный логин",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Login", mock.Anything, "test@example.com", "password123", mock.Anything).
					Return(testTokenPair, nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.TokenPair
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "test_access_token", response.AccessToken)
				assert.Equal(t, "test_refresh_token", response.RefreshToken)
				assert.Equal(t, int64(900), response.ExpiresIn)

				cookies := rec.Result().Cookies()
				assert.Len(t, cookies, 2)

				var accessCookie, refreshCookie *http.Cookie
				for _, cookie := range cookies {
					if cookie.Name == "access_token" {
						accessCookie = cookie
					}
					if cookie.Name == "refresh_token" {
						refreshCookie = cookie
					}
				}

				assert.NotNil(t, accessCookie)
				assert.Equal(t, "test_access_token", accessCookie.Value)
				assert.Equal(t, "/api", accessCookie.Path)
				assert.Equal(t, 900, accessCookie.MaxAge)

				assert.NotNil(t, refreshCookie)
				assert.Equal(t, "test_refresh_token", refreshCookie.Value)
				assert.Equal(t, "/auth/refresh", refreshCookie.Path)
				assert.Equal(t, 604800, refreshCookie.MaxAge)
			},
		},
		{
			name:               "невалидный JSON",
			requestBody:        "invalid json",
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "пустой email",
			requestBody: AuthDTO{
				Email:    "",
				Password: "password123",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Login", mock.Anything, "", "password123", mock.Anything).
					Return(nil, domain.ErrInvalidCredentials)
			},
			expectedStatusCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "невалидный формат email",
			requestBody: AuthDTO{
				Email:    "invalid-email",
				Password: "password123",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Login", mock.Anything, "invalid-email", "password123", mock.Anything).
					Return(nil, domain.ErrInvalidCredentials)
			},
			expectedStatusCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "пустой пароль",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Login", mock.Anything, "test@example.com", "", mock.Anything).
					Return(nil, domain.ErrInvalidCredentials)
			},
			expectedStatusCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "пароль короче 8 символов",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "pass123",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Login", mock.Anything, "test@example.com", "pass123", mock.Anything).
					Return(nil, domain.ErrInvalidCredentials)
			},
			expectedStatusCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "ошибка при логине - неверные креды",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Login", mock.Anything, "test@example.com", "password123", mock.Anything).
					Return(nil, errors.New("invalid credentials"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
				assert.NotContains(t, response, "message")
			},
		},
		{
			name: "email с пробелами обрезается",
			requestBody: AuthDTO{
				Email:    "  test@example.com  ",
				Password: "  password123  ",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Login", mock.Anything, "  test@example.com  ", "  password123  ", mock.Anything).
					Return(testTokenPair, nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.TokenPair
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "test_access_token", response.AccessToken)
				assert.Equal(t, "test_refresh_token", response.RefreshToken)
			},
		},
		{
			name: "email только из пробелов",
			requestBody: AuthDTO{
				Email:    "   ",
				Password: "password123",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Login", mock.Anything, "   ", "password123", mock.Anything).
					Return(nil, domain.ErrInvalidCredentials)
			},
			expectedStatusCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "error")
			},
		},
		{
			name: "пароль только из пробелов",
			requestBody: AuthDTO{
				Email:    "test@example.com",
				Password: "        ",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Login", mock.Anything, "test@example.com", "        ", mock.Anything).
					Return(testTokenPair, nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.TokenPair
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMock := mocks.NewAuthUseCase(t)
			tt.setupMock(authMock)

			handler := &handler{
				auth: authMock,
			}

			var body []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				assert.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
			req.RemoteAddr = "127.0.0.1:1234"
			req = req.WithContext(context.Background())
			rec := httptest.NewRecorder()

			handler.Login(rec, req)

			assert.Equal(t, tt.expectedStatusCode, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestRefreshHandler(t *testing.T) {
	testTokenPair := &domain.TokenPair{
		AccessToken:  "new_access_token",
		RefreshToken: "new_refresh_token",
		ExpiresIn:    900,
	}

	tests := []struct {
		name               string
		setupRequest       func(*http.Request)
		requestBody        interface{}
		setupMock          func(*mocks.AuthUseCase)
		expectedStatusCode int
		checkResponse      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "успешный refresh через cookie",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "refresh_token",
					Value: "test_refresh_token",
				})
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Refresh", mock.Anything, "test_refresh_token", mock.Anything).
					Return(testTokenPair, nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.TokenPair
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "new_access_token", response.AccessToken)
				assert.Equal(t, "new_refresh_token", response.RefreshToken)
				assert.Equal(t, int64(900), response.ExpiresIn)

				cookies := rec.Result().Cookies()
				assert.Len(t, cookies, 2)

				var accessCookie, refreshCookie *http.Cookie
				for _, cookie := range cookies {
					if cookie.Name == "access_token" {
						accessCookie = cookie
					}
					if cookie.Name == "refresh_token" {
						refreshCookie = cookie
					}
				}

				assert.NotNil(t, accessCookie)
				assert.Equal(t, "new_access_token", accessCookie.Value)
				assert.Equal(t, "/api", accessCookie.Path)
				assert.Equal(t, 900, accessCookie.MaxAge)

				assert.NotNil(t, refreshCookie)
				assert.Equal(t, "new_refresh_token", refreshCookie.Value)
				assert.Equal(t, "/auth/refresh", refreshCookie.Path)
				assert.Equal(t, 604800, refreshCookie.MaxAge)
			},
		},
		{
			name: "успешный refresh через JSON body",
			setupRequest: func(req *http.Request) {
			},
			requestBody: map[string]string{
				"refresh_token": "test_refresh_token",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Refresh", mock.Anything, "test_refresh_token", mock.Anything).
					Return(testTokenPair, nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.TokenPair
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "new_access_token", response.AccessToken)
				assert.Equal(t, "new_refresh_token", response.RefreshToken)
			},
		},
		{
			name: "отсутствует refresh token - нет cookie и body",
			setupRequest: func(req *http.Request) {
			},
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
				assert.Contains(t, response, "message")
				assert.Contains(t, response["message"], "refresh token отсутствует")
			},
		},
		{
			name: "пустой refresh token в body",
			setupRequest: func(req *http.Request) {
			},
			requestBody: map[string]string{
				"refresh_token": "",
			},
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
				assert.Contains(t, response, "message")
				assert.Contains(t, response["message"], "refresh token отсутствует")
			},
		},
		{
			name: "невалидный JSON",
			setupRequest: func(req *http.Request) {
			},
			requestBody:        "invalid json",
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
				assert.Contains(t, response, "message")
				assert.Contains(t, response["message"], "refresh token отсутствует")
			},
		},
		{
			name: "ошибка при refresh",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "refresh_token",
					Value: "invalid_refresh_token",
				})
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Refresh", mock.Anything, "invalid_refresh_token", mock.Anything).
					Return(nil, errors.New("invalid refresh token"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "приоритет cookie над body",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "refresh_token",
					Value: "cookie_token",
				})
			},
			requestBody: map[string]string{
				"refresh_token": "body_token",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Refresh", mock.Anything, "cookie_token", mock.Anything).
					Return(testTokenPair, nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.TokenPair
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "new_access_token", response.AccessToken)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMock := mocks.NewAuthUseCase(t)
			tt.setupMock(authMock)

			handler := &handler{
				auth: authMock,
			}

			var body []byte
			var err error
			if tt.requestBody != nil {
				if str, ok := tt.requestBody.(string); ok {
					body = []byte(str)
				} else {
					body, err = json.Marshal(tt.requestBody)
					assert.NoError(t, err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
			req.RemoteAddr = "127.0.0.1:1234"
			req = req.WithContext(context.Background())

			if tt.setupRequest != nil {
				tt.setupRequest(req)
			}

			rec := httptest.NewRecorder()

			handler.Refresh(rec, req)

			assert.Equal(t, tt.expectedStatusCode, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestLogoutHandler(t *testing.T) {
	tests := []struct {
		name               string
		setupRequest       func(*http.Request)
		requestBody        interface{}
		setupMock          func(*mocks.AuthUseCase)
		expectedStatusCode int
		checkResponse      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "успешный логаут через cookie",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "refresh_token",
					Value: "test_refresh_token",
				})
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Logout", mock.Anything, "test_refresh_token", mock.Anything).
					Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "success", response["message"])

				cookies := rec.Result().Cookies()
				assert.Len(t, cookies, 2)

				var accessCookie, refreshCookie *http.Cookie
				for _, cookie := range cookies {
					if cookie.Name == "access_token" {
						accessCookie = cookie
					}
					if cookie.Name == "refresh_token" {
						refreshCookie = cookie
					}
				}

				assert.NotNil(t, accessCookie)
				assert.Equal(t, "", accessCookie.Value)
				assert.Equal(t, -1, accessCookie.MaxAge)

				assert.NotNil(t, refreshCookie)
				assert.Equal(t, "", refreshCookie.Value)
				assert.Equal(t, -1, refreshCookie.MaxAge)
			},
		},
		{
			name: "успешный логаут через JSON body",
			setupRequest: func(req *http.Request) {
			},
			requestBody: map[string]string{
				"refresh_token": "test_refresh_token",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Logout", mock.Anything, "test_refresh_token", mock.Anything).
					Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "success", response["message"])

				cookies := rec.Result().Cookies()
				assert.Len(t, cookies, 2)
			},
		},
		{
			name: "отсутствует refresh token - нет cookie и body",
			setupRequest: func(req *http.Request) {
			},
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "пустой refresh token в body",
			setupRequest: func(req *http.Request) {
			},
			requestBody: map[string]string{
				"refresh_token": "",
			},
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "невалидный JSON",
			setupRequest: func(req *http.Request) {
			},
			requestBody:        "invalid json",
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "ошибка при логауте",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "refresh_token",
					Value: "test_refresh_token",
				})
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Logout", mock.Anything, "test_refresh_token", mock.Anything).
					Return(errors.New("database error"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "приоритет cookie над body",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "refresh_token",
					Value: "cookie_token",
				})
			},
			requestBody: map[string]string{
				"refresh_token": "body_token",
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("Logout", mock.Anything, "cookie_token", mock.Anything).
					Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "success", response["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMock := mocks.NewAuthUseCase(t)
			tt.setupMock(authMock)

			handler := &handler{
				auth: authMock,
			}

			var body []byte
			var err error
			if tt.requestBody != nil {
				if str, ok := tt.requestBody.(string); ok {
					body = []byte(str)
				} else {
					body, err = json.Marshal(tt.requestBody)
					assert.NoError(t, err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBuffer(body))
			req.RemoteAddr = "127.0.0.1:1234"
			req = req.WithContext(context.Background())

			if tt.setupRequest != nil {
				tt.setupRequest(req)
			}

			rec := httptest.NewRecorder()

			handler.Logout(rec, req)

			assert.Equal(t, tt.expectedStatusCode, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

func TestLogoutAllHandler(t *testing.T) {
	testUserID := uuid.New()

	tests := []struct {
		name               string
		setupContext       func(context.Context) context.Context
		setupMock          func(*mocks.AuthUseCase)
		expectedStatusCode int
		checkResponse      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "успешный logout всех сессий",
			setupContext: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "user_id", testUserID.String())
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("LogoutAll", mock.Anything, testUserID).
					Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "success", response["message"])

				cookies := rec.Result().Cookies()
				assert.Len(t, cookies, 2)

				var accessCookie, refreshCookie *http.Cookie
				for _, cookie := range cookies {
					if cookie.Name == "access_token" {
						accessCookie = cookie
					}
					if cookie.Name == "refresh_token" {
						refreshCookie = cookie
					}
				}

				assert.NotNil(t, accessCookie)
				assert.Equal(t, "", accessCookie.Value)
				assert.Equal(t, -1, accessCookie.MaxAge)

				assert.NotNil(t, refreshCookie)
				assert.Equal(t, "", refreshCookie.Value)
				assert.Equal(t, -1, refreshCookie.MaxAge)
			},
		},
		{
			name: "отсутствует user_id в контексте",
			setupContext: func(ctx context.Context) context.Context {
				return ctx
			},
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
				assert.Contains(t, response, "message")
				assert.Contains(t, response["message"], "поле user_id должно быть string")
			},
		},
		{
			name: "user_id не является строкой",
			setupContext: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "user_id", 12345)
			},
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
				assert.Contains(t, response, "message")
				assert.Contains(t, response["message"], "поле user_id должно быть string")
			},
		},
		{
			name: "невалидный формат UUID",
			setupContext: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "user_id", "invalid-uuid-format")
			},
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "пустая строка вместо UUID",
			setupContext: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "user_id", "")
			},
			setupMock:          func(authMock *mocks.AuthUseCase) {},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "ошибка при LogoutAll в usecase",
			setupContext: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "user_id", testUserID.String())
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("LogoutAll", mock.Anything, testUserID).
					Return(errors.New("database connection error"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "валидный UUID с другим пользователем",
			setupContext: func(ctx context.Context) context.Context {
				anotherUserID := uuid.New()
				return context.WithValue(ctx, "user_id", anotherUserID.String())
			},
			setupMock: func(authMock *mocks.AuthUseCase) {
				authMock.On("LogoutAll", mock.Anything, mock.AnythingOfType("uuid.UUID")).
					Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.NewDecoder(rec.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "success", response["message"])

				cookies := rec.Result().Cookies()
				assert.Len(t, cookies, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMock := mocks.NewAuthUseCase(t)
			tt.setupMock(authMock)

			handler := &handler{
				auth: authMock,
			}

			req := httptest.NewRequest(http.MethodPost, "/auth/logout-all", nil)
			req.RemoteAddr = "127.0.0.1:1234"

			ctx := context.Background()
			if tt.setupContext != nil {
				ctx = tt.setupContext(ctx)
			}
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()

			handler.LogoutAll(rec, req)

			assert.Equal(t, tt.expectedStatusCode, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}
