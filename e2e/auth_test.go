package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthFlow(t *testing.T) {
	ts := SetupTestServer(t)

	t.Run("успешная регистрация и логин", func(t *testing.T) {
		registerPayload := map[string]string{
			"email":    "test@example.com",
			"password": "password123",
			"username": "TestUser",
		}
		body, _ := json.Marshal(registerPayload)

		resp, err := http.Post(ts.Server.URL+"/auth/register", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var registerResp struct {
			Account struct {
				ID       string `json:"id"`
				Email    string `json:"email"`
				Username string `json:"username"`
			} `json:"account"`
			Tokens struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			} `json:"tokens"`
		}
		err = json.NewDecoder(resp.Body).Decode(&registerResp)
		require.NoError(t, err)

		assert.NotEmpty(t, registerResp.Account.ID)
		assert.Equal(t, "test@example.com", registerResp.Account.Email)
		assert.Equal(t, "TestUser", registerResp.Account.Username)
		assert.NotEmpty(t, registerResp.Tokens.AccessToken)
		assert.NotEmpty(t, registerResp.Tokens.RefreshToken)

		var count int
		err = ts.DB.QueryRow("SELECT COUNT(*) FROM accounts WHERE email = $1", "test@example.com").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("дублирующая регистрация", func(t *testing.T) {
		email := "duplicate@example.com"

		registerPayload := map[string]string{
			"email":    email,
			"password": "password123",
			"username": "User1",
		}
		body, _ := json.Marshal(registerPayload)
		resp1, err := http.Post(ts.Server.URL+"/auth/register", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		resp1.Body.Close()
		assert.Equal(t, http.StatusCreated, resp1.StatusCode)

		resp2, err := http.Post(ts.Server.URL+"/auth/register", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp2.Body.Close()

		assert.NotEqual(t, http.StatusCreated, resp2.StatusCode)
	})

	t.Run("логин с корректными данными", func(t *testing.T) {
		email := "login@example.com"
		password := "mypassword"

		registerPayload := map[string]string{
			"email":    email,
			"password": password,
			"username": "LoginUser",
		}
		body, _ := json.Marshal(registerPayload)
		resp, err := http.Post(ts.Server.URL+"/auth/register", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		resp.Body.Close()

		loginPayload := map[string]string{
			"email":    email,
			"password": password,
		}
		body, _ = json.Marshal(loginPayload)
		resp, err = http.Post(ts.Server.URL+"/auth/login", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var loginResp struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		err = json.NewDecoder(resp.Body).Decode(&loginResp)
		require.NoError(t, err)

		assert.NotEmpty(t, loginResp.AccessToken)
		assert.NotEmpty(t, loginResp.RefreshToken)
	})

	t.Run("логин с неверным паролем", func(t *testing.T) {
		email := "wrongpass@example.com"

		registerPayload := map[string]string{
			"email":    email,
			"password": "correctpassword",
			"username": "WrongPassUser",
		}
		body, _ := json.Marshal(registerPayload)
		resp, err := http.Post(ts.Server.URL+"/auth/register", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		resp.Body.Close()

		loginPayload := map[string]string{
			"email":    email,
			"password": "wrongpassword",
		}
		body, _ = json.Marshal(loginPayload)
		resp, err = http.Post(ts.Server.URL+"/auth/login", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("refresh token", func(t *testing.T) {
		email := "refresh@example.com"
		registerPayload := map[string]string{
			"email":    email,
			"password": "password123",
			"username": "RefreshUser",
		}
		body, _ := json.Marshal(registerPayload)
		resp, err := http.Post(ts.Server.URL+"/auth/register", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)

		var registerResp struct {
			Tokens struct {
				RefreshToken string `json:"refresh_token"`
			} `json:"tokens"`
		}
		json.NewDecoder(resp.Body).Decode(&registerResp)
		resp.Body.Close()

		refreshToken := registerResp.Tokens.RefreshToken

		refreshPayload := map[string]string{
			"refresh_token": refreshToken,
		}
		body, _ = json.Marshal(refreshPayload)
		resp, err = http.Post(ts.Server.URL+"/auth/refresh", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var refreshResp struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		err = json.NewDecoder(resp.Body).Decode(&refreshResp)
		require.NoError(t, err)

		assert.NotEmpty(t, refreshResp.AccessToken)
		assert.NotEmpty(t, refreshResp.RefreshToken)
	})

	t.Run("logout", func(t *testing.T) {
		email := "logout@example.com"
		registerPayload := map[string]string{
			"email":    email,
			"password": "password123",
			"username": "LogoutUser",
		}
		body, _ := json.Marshal(registerPayload)
		resp, err := http.Post(ts.Server.URL+"/auth/register", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)

		var registerResp struct {
			Tokens struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			} `json:"tokens"`
		}
		json.NewDecoder(resp.Body).Decode(&registerResp)
		resp.Body.Close()

		logoutPayload := map[string]string{
			"refresh_token": registerResp.Tokens.RefreshToken,
		}
		body, _ = json.Marshal(logoutPayload)
		req, _ := http.NewRequest("POST", ts.Server.URL+"/auth/logout", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		refreshPayload := map[string]string{
			"refresh_token": registerResp.Tokens.RefreshToken,
		}
		body, _ = json.Marshal(refreshPayload)
		resp, err = http.Post(ts.Server.URL+"/auth/refresh", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})
}

func TestLogoutAll(t *testing.T) {
	ts := SetupTestServer(t)

	email := "logoutall@example.com"
	registerPayload := map[string]string{
		"email":    email,
		"password": "password123",
		"username": "LogoutAllUser",
	}
	body, _ := json.Marshal(registerPayload)
	resp, err := http.Post(ts.Server.URL+"/auth/register", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)

	var registerResp struct {
		Tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	}
	json.NewDecoder(resp.Body).Decode(&registerResp)
	resp.Body.Close()

	firstRefreshToken := registerResp.Tokens.RefreshToken

	refreshPayload := map[string]string{
		"refresh_token": firstRefreshToken,
	}
	body, _ = json.Marshal(refreshPayload)
	resp, err = http.Post(ts.Server.URL+"/auth/refresh", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)

	var refreshResp struct {
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(resp.Body).Decode(&refreshResp)
	resp.Body.Close()

	secondRefreshToken := refreshResp.RefreshToken

	req, _ := http.NewRequest("POST", ts.Server.URL+"/auth/logout-all", nil)
	req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)

	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	for _, token := range []string{firstRefreshToken, secondRefreshToken} {
		refreshPayload := map[string]string{
			"refresh_token": token,
		}
		body, _ = json.Marshal(refreshPayload)
		resp, err = http.Post(ts.Server.URL+"/auth/refresh", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		resp.Body.Close()

		assert.NotEqual(t, http.StatusOK, resp.StatusCode, "токен %s должен быть невалидным", token)
	}
}

func TestInvalidToken(t *testing.T) {
	ts := SetupTestServer(t)

	accountID := uuid.New().String()
	req, _ := http.NewRequest("GET", ts.Server.URL+"/accounts/"+accountID, nil)
	req.Header.Set("Authorization", "Bearer invalid_token_here")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestMissingToken(t *testing.T) {
	ts := SetupTestServer(t)

	accountID := uuid.New().String()
	req, _ := http.NewRequest("GET", ts.Server.URL+"/accounts/"+accountID, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
