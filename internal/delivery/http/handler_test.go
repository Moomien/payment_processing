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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTransferHandler(t *testing.T) {
	tests := []struct {
		name               string
		requestBody        interface{}
		idempotencyKey     string
		setupMock          func(*mocks.TransactionUsecase, uuid.UUID, uuid.UUID, decimal.Decimal)
		expectedStatusCode int
		expectError        bool
	}{
		{
			name: "успешный перевод",
			requestBody: transferDTO{
				Sender_id:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
				Amount:      "500.50",
			},
			idempotencyKey: "test-key-123",
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				m.On("Transfer",
					mock.Anything,
					senderID,
					receiverID,
					"test-key-123",
					amount,
				).Return(nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:        "невалидный JSON",
			requestBody: `{"invalid json`,
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				// не вызываем Transfer, т.к. ошибка парсинга раньше
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name: "невалидный amount формат",
			requestBody: transferDTO{
				Sender_id:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
				Amount:      "invalid-amount",
			},
			idempotencyKey: "test-key-456",
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				// не вызываем Transfer, т.к. ошибка парсинга amount
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name: "ошибка от usecase",
			requestBody: transferDTO{
				Sender_id:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
				Amount:      "1000.00",
			},
			idempotencyKey: "test-key-789",
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				m.On("Transfer",
					mock.Anything,
					senderID,
					receiverID,
					"test-key-789",
					amount,
				).Return(errors.New("недостаточно средств")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name: "пустой idempotency key",
			requestBody: transferDTO{
				Sender_id:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
				Amount:      "100.00",
			},
			idempotencyKey: "",
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				m.On("Transfer",
					mock.Anything,
					senderID,
					receiverID,
					"",
					amount,
				).Return(nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUsecase := mocks.NewTransactionUsecase(t)

			var senderID, receiverID uuid.UUID
			var amount decimal.Decimal
			if dto, ok := tt.requestBody.(transferDTO); ok {
				senderID = dto.Sender_id
				receiverID = dto.Receiver_id
				amount, _ = decimal.NewFromString(dto.Amount)
			}

			tt.setupMock(mockUsecase, senderID, receiverID, amount)

			handler := NewHandler(mockUsecase)

			var bodyBytes []byte
			var err error
			if strBody, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(strBody)
			} else {
				bodyBytes, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			if tt.idempotencyKey != "" {
				req.Header.Set("Idempotency-Key", tt.idempotencyKey)
			}
			req = req.WithContext(context.Background())
			rr := httptest.NewRecorder()
			handler.Transfer(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "неожиданный статус код")
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			if tt.expectError {
				assert.NotEmpty(t, rr.Body.String(), "ожидался response body с ошибкой")
			}
		})
	}
}
