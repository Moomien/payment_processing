package e2e

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAccount(t *testing.T) {
	ts := SetupTestServer(t)

	user := createTestUser(t, ts, "getaccount@example.com", "password123", "AccountUser")

	t.Run("получение своего аккаунта", func(t *testing.T) {
		url := fmt.Sprintf("%s/accounts/%s", ts.Server.URL, user.AccountID)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+user.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("получение чужого аккаунта", func(t *testing.T) {
		otherUser := createTestUser(t, ts, "other@example.com", "password123", "OtherUser")

		url := fmt.Sprintf("%s/accounts/%s", ts.Server.URL, otherUser.AccountID)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+user.AccessToken) // токен user, запрашиваем otherUser

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("получение несуществующего аккаунта", func(t *testing.T) {
		fakeID := uuid.New().String()
		url := fmt.Sprintf("%s/accounts/%s", ts.Server.URL, fakeID)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+user.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("получение аккаунта без токена", func(t *testing.T) {
		url := fmt.Sprintf("%s/accounts/%s", ts.Server.URL, user.AccountID)
		req, _ := http.NewRequest("GET", url, nil)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
