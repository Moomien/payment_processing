package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionFlow(t *testing.T) {
	ts := SetupTestServer(t)

	// Создаем двух пользователей
	sender := createTestUser(t, ts, "sender@example.com", "password123", "Sender")
	receiver := createTestUser(t, ts, "receiver@example.com", "password123", "Receiver")

	// Добавляем баланс отправителю напрямую в БД
	_, err := ts.DB.Exec("UPDATE accounts SET balance = 1000 WHERE id = $1", sender.AccountID)
	require.NoError(t, err)

	t.Run("успешный перевод", func(t *testing.T) {
		transferPayload := map[string]interface{}{
			"receiver_id": receiver.AccountID,
			"amount":      "100.50",
		}
		body, _ := json.Marshal(transferPayload)

		req, _ := http.NewRequest("POST", ts.Server.URL+"/transactions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+sender.AccessToken)
		req.Header.Set("Idempotency-Key", "transaction-flow-success")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var transactionResp struct {
			TransactionID string `json:"transaction_id"`
		}
		err = json.NewDecoder(resp.Body).Decode(&transactionResp)
		require.NoError(t, err)
		assert.NotEmpty(t, transactionResp.TransactionID)

		// Проверяем балансы в БД
		var senderBalance, receiverBalance float64
		err = ts.DB.QueryRow("SELECT balance FROM accounts WHERE id = $1", sender.AccountID).Scan(&senderBalance)
		require.NoError(t, err)
		assert.InDelta(t, 899.50, senderBalance, 0.01)

		err = ts.DB.QueryRow("SELECT balance FROM accounts WHERE id = $1", receiver.AccountID).Scan(&receiverBalance)
		require.NoError(t, err)
		assert.InDelta(t, 100.50, receiverBalance, 0.01)

		// Проверяем, что транзакция записалась в БД
		var status string
		err = ts.DB.QueryRow("SELECT status FROM transactions WHERE id = $1", transactionResp.TransactionID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "completed", status)
	})

	t.Run("перевод с недостаточным балансом", func(t *testing.T) {
		transferPayload := map[string]interface{}{
			"receiver_id": receiver.AccountID,
			"amount":      "10000.00", // Больше, чем есть на счету
		}
		body, _ := json.Marshal(transferPayload)

		req, _ := http.NewRequest("POST", ts.Server.URL+"/transactions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+sender.AccessToken)
		req.Header.Set("Idempotency-Key", "transaction-flow-insufficient-funds")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("перевод самому себе", func(t *testing.T) {
		transferPayload := map[string]interface{}{
			"receiver_id": sender.AccountID, // Отправитель = получатель
			"amount":      "50.00",
		}
		body, _ := json.Marshal(transferPayload)

		req, _ := http.NewRequest("POST", ts.Server.URL+"/transactions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+sender.AccessToken)
		req.Header.Set("Idempotency-Key", "transaction-flow-same-account")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("перевод с отрицательной суммой", func(t *testing.T) {
		transferPayload := map[string]interface{}{
			"receiver_id": receiver.AccountID,
			"amount":      "-10.00",
		}
		body, _ := json.Marshal(transferPayload)

		req, _ := http.NewRequest("POST", ts.Server.URL+"/transactions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+sender.AccessToken)
		req.Header.Set("Idempotency-Key", "transaction-flow-negative-amount")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("перевод несуществующему получателю", func(t *testing.T) {
		fakeReceiverID := uuid.New().String()
		transferPayload := map[string]interface{}{
			"receiver_id": fakeReceiverID,
			"amount":      "10.00",
		}
		body, _ := json.Marshal(transferPayload)

		req, _ := http.NewRequest("POST", ts.Server.URL+"/transactions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+sender.AccessToken)
		req.Header.Set("Idempotency-Key", "transaction-flow-missing-receiver")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusCreated, resp.StatusCode)
	})
}

func TestGetTransaction(t *testing.T) {
	ts := SetupTestServer(t)

	// Создаем пользователей и делаем транзакцию
	sender := createTestUser(t, ts, "getsender@example.com", "password123", "GetSender")
	receiver := createTestUser(t, ts, "getreceiver@example.com", "password123", "GetReceiver")

	// Добавляем баланс
	_, err := ts.DB.Exec("UPDATE accounts SET balance = 500 WHERE id = $1", sender.AccountID)
	require.NoError(t, err)

	// Создаем транзакцию
	transferPayload := map[string]interface{}{
		"receiver_id": receiver.AccountID,
		"amount":      "50.00",
	}
	body, _ := json.Marshal(transferPayload)

	req, _ := http.NewRequest("POST", ts.Server.URL+"/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sender.AccessToken)
	req.Header.Set("Idempotency-Key", "get-transaction-setup")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	var transactionResp struct {
		TransactionID string `json:"transaction_id"`
	}
	json.NewDecoder(resp.Body).Decode(&transactionResp)
	resp.Body.Close()

	transactionID := transactionResp.TransactionID

	t.Run("получение транзакции отправителем", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.Server.URL+"/transactions/"+transactionID, nil)
		req.Header.Set("Authorization", "Bearer "+sender.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var tx struct {
			Amount     string `json:"amount"`
			SenderID   string `json:"sender_id"`
			ReceiverID string `json:"receiver_id"`
			Status     string `json:"status"`
		}
		err = json.NewDecoder(resp.Body).Decode(&tx)
		require.NoError(t, err)

		assert.Equal(t, "50", tx.Amount)
		assert.Equal(t, sender.AccountID, tx.SenderID)
		assert.Equal(t, receiver.AccountID, tx.ReceiverID)
		assert.Equal(t, "completed", tx.Status)
	})

	t.Run("получение транзакции получателем", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.Server.URL+"/transactions/"+transactionID, nil)
		req.Header.Set("Authorization", "Bearer "+receiver.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("получение чужой транзакции", func(t *testing.T) {
		stranger := createTestUser(t, ts, "stranger@example.com", "password123", "Stranger")

		req, _ := http.NewRequest("GET", ts.Server.URL+"/transactions/"+transactionID, nil)
		req.Header.Set("Authorization", "Bearer "+stranger.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAccountTransactionHistory(t *testing.T) {
	ts := SetupTestServer(t)

	user1 := createTestUser(t, ts, "history1@example.com", "password123", "HistoryUser1")
	user2 := createTestUser(t, ts, "history2@example.com", "password123", "HistoryUser2")
	user3 := createTestUser(t, ts, "history3@example.com", "password123", "HistoryUser3")

	// Добавляем баланс
	_, err := ts.DB.Exec("UPDATE accounts SET balance = 1000 WHERE id = $1", user1.AccountID)
	require.NoError(t, err)

	// Создаем несколько транзакций
	for i := 0; i < 5; i++ {
		receiver := user2.AccountID
		if i%2 == 0 {
			receiver = user3.AccountID
		}

		transferPayload := map[string]interface{}{
			"receiver_id": receiver,
			"amount":      fmt.Sprintf("%d.00", 10*(i+1)),
		}
		body, _ := json.Marshal(transferPayload)

		req, _ := http.NewRequest("POST", ts.Server.URL+"/transactions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+user1.AccessToken)
		req.Header.Set("Idempotency-Key", fmt.Sprintf("test-transaction-%d", i)) // уникальный ключ для каждого запроса

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
	}

	t.Run("получение истории транзакций", func(t *testing.T) {
		url := fmt.Sprintf("%s/accounts/%s/transactions?limit=10&offset=0", ts.Server.URL, user1.AccountID)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+user1.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var historyResp struct {
			Total        int `json:"total"`
			Transactions []struct {
				Amount     string `json:"amount"`
				SenderID   string `json:"sender_id"`
				ReceiverID string `json:"receiver_id"`
				Status     string `json:"status"`
			} `json:"transactions"`
		}
		err = json.NewDecoder(resp.Body).Decode(&historyResp)
		require.NoError(t, err)

		assert.Equal(t, 5, historyResp.Total)
		assert.Len(t, historyResp.Transactions, 5)
	})

	t.Run("пагинация истории транзакций", func(t *testing.T) {
		url := fmt.Sprintf("%s/accounts/%s/transactions?limit=2&offset=0", ts.Server.URL, user1.AccountID)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+user1.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		var historyResp struct {
			Total        int           `json:"total"`
			Transactions []interface{} `json:"transactions"`
		}
		err = json.NewDecoder(resp.Body).Decode(&historyResp)
		require.NoError(t, err)

		assert.Equal(t, 5, historyResp.Total)
		assert.Len(t, historyResp.Transactions, 2)
	})

	t.Run("получение истории чужого аккаунта", func(t *testing.T) {
		url := fmt.Sprintf("%s/accounts/%s/transactions?limit=10&offset=0", ts.Server.URL, user2.AccountID)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+user1.AccessToken) // токен user1, но запрашиваем историю user2

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})
}
