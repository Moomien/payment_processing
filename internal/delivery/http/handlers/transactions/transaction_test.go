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
	"processing/internal/delivery/http/requestctx"
	"processing/internal/domain"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTransactionTransferHandler(t *testing.T) {
	testSenderID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

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
				).Return("test-transaction-id", nil).Once()
			},
			expectedStatusCode: http.StatusCreated,
			expectError:        false,
		},
		{
			name:        "невалидный JSON",
			requestBody: `{"invalid json`,
			idempotencyKey: "test-key-invalid-json",
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				// не вызываем Transfer, т.к. ошибка парсинга раньше
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name: "невалидный amount формат",
			requestBody: transferDTO{
				Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
				Amount:      "invalid-amount",
			},
			idempotencyKey: "test-key-456",
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				// не вызываем Transfer, т.к. ошибка парсинга amount
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name: "ошибка от usecase",
			requestBody: transferDTO{
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
				).Return("", errors.New("недостаточно средств")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name: "пустой idempotency key",
			requestBody: transferDTO{
				Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
				Amount:      "100.00",
			},
			idempotencyKey: "",
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				// Transfer не вызывается: обязательный заголовок проверяется раньше.
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name: "idempotency key с недопустимыми символами",
			requestBody: transferDTO{
				Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
				Amount:      "100.00",
			},
			idempotencyKey: "invalid key",
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				// Transfer не вызывается: ключ проверяется раньше.
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name: "слишком длинный idempotency key",
			requestBody: transferDTO{
				Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
				Amount:      "100.00",
			},
			idempotencyKey: strings.Repeat("a", domain.MaxIdempotencyKeyLength+1),
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				// Transfer не вызывается: ключ проверяется раньше.
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name: "конфликт idempotency key",
			requestBody: transferDTO{
				Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
				Amount:      "100.00",
			},
			idempotencyKey: "conflict-key",
			setupMock: func(m *mocks.TransactionUsecase, senderID, receiverID uuid.UUID, amount decimal.Decimal) {
				m.On("Transfer",
					mock.Anything,
					senderID,
					receiverID,
					"conflict-key",
					amount,
				).Return("", domain.ErrIdempotencyConflict).Once()
			},
			expectedStatusCode: http.StatusConflict,
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUsecase := mocks.NewTransactionUsecase(t)
			mockAccountUsecase := mocks.NewAccountsUsecase(t)
			mockAuthUsecase := mocks.NewAuthUseCase(t)
			handler := NewHandler(mockUsecase, mockAccountUsecase, mockAuthUsecase, nil)

			var receiverID uuid.UUID
			var amount decimal.Decimal
			if dto, ok := tt.requestBody.(transferDTO); ok {
				receiverID = dto.Receiver_id
				amount, _ = decimal.NewFromString(dto.Amount)
			}

			tt.setupMock(mockUsecase, testSenderID, receiverID, amount)

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
			ctx := requestctx.WithIdentity(context.Background(), requestctx.Identity{UserID: testSenderID.String()})
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()
			handler.Transfer(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "неожиданный статус код")
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			if tt.expectError {
				assert.NotEmpty(t, rr.Body.String(), "ожидался response body с ошибкой")
			} else {
				var resp map[string]string
				err = json.Unmarshal(rr.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.NotEmpty(t, resp["transaction_id"])
			}
		})
		t.Log("\n\n\n")
	}
}

func TestGetTransactionHandler(t *testing.T) {
	validTransactionID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	testUserID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174001")

	tests := []struct {
		name               string
		transactionID      string
		setupMock          func(*mocks.TransactionUsecase)
		expectedStatusCode int
		expectError        bool
	}{
		{
			name:          "успешное получение транзакции",
			transactionID: validTransactionID.String(),
			setupMock: func(m *mocks.TransactionUsecase) {
				amount, _ := decimal.NewFromString("500.50")
				expectedTransaction := domain.Transaction{
					ID:          validTransactionID,
					Amount:      amount,
					Sender_id:   testUserID,
					Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174002"),
					Status:      domain.StatusCompleted,
					Created_at:  time.Now(),
				}
				m.On("GetTransaction",
					mock.Anything,
					validTransactionID,
					testUserID,
					"",
				).Return(expectedTransaction, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:          "невалидный UUID транзакции",
			transactionID: "invalid-uuid",
			setupMock: func(m *mocks.TransactionUsecase) {
				// не вызываем GetTransaction, т.к. ошибка парсинга UUID раньше
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:          "транзакция не найдена",
			transactionID: validTransactionID.String(),
			setupMock: func(m *mocks.TransactionUsecase) {
				m.On("GetTransaction",
					mock.Anything,
					validTransactionID,
					testUserID,
					"",
				).Return(domain.Transaction{}, errors.New("транзакция не найдена")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:          "доступ запрещен к чужой транзакции",
			transactionID: validTransactionID.String(),
			setupMock: func(m *mocks.TransactionUsecase) {
				m.On("GetTransaction",
					mock.Anything,
					validTransactionID,
					testUserID,
					"",
				).Return(domain.Transaction{}, domain.ErrAccessDenied).Once()
			},
			expectedStatusCode: http.StatusNotFound,
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUsecase := mocks.NewTransactionUsecase(t)
			mockAccountUsecase := mocks.NewAccountsUsecase(t)
			mockAuthUsecae := mocks.NewAuthUseCase(t)
			handler := NewHandler(mockUsecase, mockAccountUsecase, mockAuthUsecae, nil)

			tt.setupMock(mockUsecase)

			req := httptest.NewRequest(http.MethodGet, "/transactions/"+tt.transactionID, nil)
			req.SetPathValue("id", tt.transactionID)
			ctx := requestctx.WithIdentity(context.Background(), requestctx.Identity{UserID: testUserID.String()})
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler.GetTransaction(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "неожиданный статус код")
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			if tt.expectError {
				assert.NotEmpty(t, rr.Body.String(), "ожидался response body с ошибкой")
			} else {
				var response domain.Transaction
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				assert.NoError(t, err, "ответ должен быть валидным JSON")
			}
		})
	}
}

func TestTransactionFilterHandler(t *testing.T) {
	testUserID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174001")
	validSenderID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174002")
	validReceiverID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174003")
	amount, _ := decimal.NewFromString("500.50")
	amount2, _ := decimal.NewFromString("250")
	tests := []struct {
		name               string
		queryParams        string
		idempotencyKey     string
		setupMock          func(*mocks.TransactionUsecase)
		expectedStatusCode int
		expectError        bool
	}{
		{
			name:           "успешная фильтрация с sender_id",
			queryParams:    "sender_id=" + validSenderID.String() + "&limit=10&offset=0",
			idempotencyKey: "test-key-123",
			setupMock: func(m *mocks.TransactionUsecase) {
				expectedTransactions := []domain.Transaction{
					{
						ID:          uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
						Amount:      amount,
						Sender_id:   validSenderID,
						Receiver_id: validReceiverID,
						Status:      domain.StatusCompleted,
						Created_at:  time.Now(),
					},
				}
				m.On("GetTransactionFilter",
					mock.Anything,
					mock.MatchedBy(func(filter *domain.TransactionFilter) bool {
						return filter.SenderID == validSenderID &&
							filter.Limit == 10 &&
							filter.Offset == 0
					}),
					testUserID,
					"test-key-123",
				).Return(expectedTransactions, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:           "успешная фильтрация с receiver_id и датами",
			queryParams:    "receiver_id=" + validReceiverID.String() + "&from=2024-01-01&to=2024-12-31&limit=20&offset=5",
			idempotencyKey: "test-key-456",
			setupMock: func(m *mocks.TransactionUsecase) {
				expectedTransactions := []domain.Transaction{}
				m.On("GetTransactionFilter",
					mock.Anything,
					mock.MatchedBy(func(filter *domain.TransactionFilter) bool {
						return filter.ReceiverID == validReceiverID &&
							filter.Limit == 20 &&
							filter.Offset == 5
					}),
					testUserID,
					"test-key-456",
				).Return(expectedTransactions, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:           "фильтрация с min_amount и max_amount",
			queryParams:    "min_amount=100.00&max_amount=1000.00&limit=15&offset=0",
			idempotencyKey: "test-key-789",
			setupMock: func(m *mocks.TransactionUsecase) {
				expectedTransactions := []domain.Transaction{
					{
						ID:          uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
						Amount:      amount2,
						Sender_id:   validSenderID,
						Receiver_id: validReceiverID,
						Status:      domain.StatusCompleted,
						Created_at:  time.Now(),
					},
				}
				m.On("GetTransactionFilter",
					mock.Anything,
					mock.MatchedBy(func(filter *domain.TransactionFilter) bool {
						return filter.MinAmount == "100.00" &&
							filter.MaxAmount == "1000.00" &&
							filter.Limit == 15 &&
							filter.Offset == 0
					}),
					testUserID,
					"test-key-789",
				).Return(expectedTransactions, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:        "невалидный limit параметр",
			queryParams: "limit=invalid&offset=0",
			setupMock: func(m *mocks.TransactionUsecase) {
				// не вызываем GetTransactionFilter, т.к. ошибка парсинга limit
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:        "невалидный UUID в sender_id",
			queryParams: "sender_id=invalid-uuid&limit=10&offset=0",
			setupMock: func(m *mocks.TransactionUsecase) {
				// не вызываем GetTransactionFilter, т.к. ошибка парсинга UUID
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:           "ошибка от usecase",
			queryParams:    "sender_id=" + validSenderID.String() + "&limit=10&offset=0",
			idempotencyKey: "test-key-777",
			setupMock: func(m *mocks.TransactionUsecase) {
				m.On("GetTransactionFilter",
					mock.Anything,
					mock.Anything,
					testUserID,
					"test-key-777",
				).Return([]domain.Transaction{}, errors.New("database error")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUsecase := mocks.NewTransactionUsecase(t)
			mockAccountUsecase := mocks.NewAccountsUsecase(t)
			mockAuthUsecae := mocks.NewAuthUseCase(t)
			handler := NewHandler(mockUsecase, mockAccountUsecase, mockAuthUsecae, nil)

			tt.setupMock(mockUsecase)

			req := httptest.NewRequest(http.MethodGet, "/transactions?"+tt.queryParams, nil)
			if tt.idempotencyKey != "" {
				req.Header.Set("Idempotency-Key", tt.idempotencyKey)
			}
			ctx := requestctx.WithIdentity(context.Background(), requestctx.Identity{UserID: testUserID.String()})
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler.TransactionFilter(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "неожиданный статус код")
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			if tt.expectError {
				assert.NotEmpty(t, rr.Body.String(), "ожидался response body с ошибкой")
			} else {
				var response []domain.Transaction
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				assert.NoError(t, err, "ответ должен быть валидным JSON")
			}
		})
	}
}
