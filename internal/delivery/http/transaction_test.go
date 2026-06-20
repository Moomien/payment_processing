package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"processing/internal/decimal"
	"processing/internal/delivery/http/mocks"
	"processing/internal/domain"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTransactionTransferHandler(t *testing.T) {
	service, err := os.OpenFile("service.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer service.Close()

	serlog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, service), nil))

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
				).Return("test-transaction-id", nil).Once()
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
				).Return("", errors.New("недостаточно средств")).Once()
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
				).Return("test-transaction-id-2", nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUsecase := mocks.NewTransactionUsecase(t)
			mockAccountUsecase := mocks.NewAccountsUsecase(t)
			mockAuthUsecase := mocks.NewAuthUseCase(t)
			handler := NewHandler(mockUsecase, mockAccountUsecase, mockAuthUsecase, serlog)

			var senderID, receiverID uuid.UUID
			var amount decimal.Decimal
			if dto, ok := tt.requestBody.(transferDTO); ok {
				senderID = dto.Sender_id
				receiverID = dto.Receiver_id
				amount, _ = decimal.NewFromString(dto.Amount)
			}

			tt.setupMock(mockUsecase, senderID, receiverID, amount)

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
		t.Log("\n\n\n")
	}
}

func TestGetTransactionHandler(t *testing.T) {
	service, err := os.OpenFile("service.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer service.Close()

	serlog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, service), nil))

	validTransactionID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	validUserID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174001")

	tests := []struct {
		name               string
		transactionID      string
		requestBody        interface{}
		setupMock          func(*mocks.TransactionUsecase)
		expectedStatusCode int
		expectError        bool
	}{
		{
			name:          "успешное получение транзакции",
			transactionID: validTransactionID.String(),
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-123",
			},
			setupMock: func(m *mocks.TransactionUsecase) {
				amount, _ := decimal.NewFromString("500.50")
				expectedTransaction := domain.Transaction{
					ID:          validTransactionID,
					Amount:      amount,
					Sender_id:   validUserID,
					Receiver_id: uuid.MustParse("123e4567-e89b-12d3-a456-426614174002"),
					Status:      domain.StatusCompleted,
					Created_at:  time.Now(),
				}
				m.On("GetTransaction",
					mock.Anything,
					validTransactionID,
					validUserID,
					"test-key-123",
				).Return(expectedTransaction, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:          "невалидный UUID транзакции",
			transactionID: "invalid-uuid",
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-456",
			},
			setupMock: func(m *mocks.TransactionUsecase) {
				// не вызываем GetTransaction, т.к. ошибка парсинга UUID раньше
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:          "невалидный JSON body",
			transactionID: validTransactionID.String(),
			requestBody:   `{"invalid json`,
			setupMock: func(m *mocks.TransactionUsecase) {
				// не вызываем GetTransaction, т.к. ошибка парсинга JSON
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:          "транзакция не найдена",
			transactionID: validTransactionID.String(),
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-789",
			},
			setupMock: func(m *mocks.TransactionUsecase) {
				m.On("GetTransaction",
					mock.Anything,
					validTransactionID,
					validUserID,
					"test-key-789",
				).Return(domain.Transaction{}, errors.New("транзакция не найдена")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:          "доступ запрещен к чужой транзакции",
			transactionID: validTransactionID.String(),
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-999",
			},
			setupMock: func(m *mocks.TransactionUsecase) {
				m.On("GetTransaction",
					mock.Anything,
					validTransactionID,
					validUserID,
					"test-key-999",
				).Return(domain.Transaction{}, errors.New("доступ запрещен")).Once()
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
			handler := NewHandler(mockUsecase, mockAccountUsecase, mockAuthUsecae, serlog)

			tt.setupMock(mockUsecase)

			var bodyBytes []byte
			var err error
			if strBody, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(strBody)
			} else {
				bodyBytes, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodGet, "/transactions/"+tt.transactionID, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", tt.transactionID)
			req = req.WithContext(context.Background())

			rr := httptest.NewRecorder()
			handler.GetTransaction(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "неожиданный статус код")
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			if tt.expectError {
				assert.NotEmpty(t, rr.Body.String(), "ожидался response body с ошибкой")
			} else {
				// проверяем что ответ содержит валидный JSON с транзакцией
				var response domain.Transaction
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				assert.NoError(t, err, "ответ должен быть валидным JSON")
			}
		})
	}
}

func TestTransactionFilterHandler(t *testing.T) {
	service, err := os.OpenFile("service.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer service.Close()

	serlog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, service), nil))

	validUserID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174001")
	validSenderID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174002")
	validReceiverID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174003")
	amount, _ := decimal.NewFromString("500.50")
	amount2, _ := decimal.NewFromString("250")
	tests := []struct {
		name               string
		queryParams        string
		requestBody        interface{}
		setupMock          func(*mocks.TransactionUsecase)
		expectedStatusCode int
		expectError        bool
	}{
		{
			name:        "успешная фильтрация с sender_id",
			queryParams: "sender_id=" + validSenderID.String() + "&limit=10&offset=0",
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-123",
			},
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
					validUserID,
					"test-key-123",
				).Return(expectedTransactions, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:        "успешная фильтрация с receiver_id и датами",
			queryParams: "receiver_id=" + validReceiverID.String() + "&from=2024-01-01&to=2024-12-31&limit=20&offset=5",
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-456",
			},
			setupMock: func(m *mocks.TransactionUsecase) {
				expectedTransactions := []domain.Transaction{}
				m.On("GetTransactionFilter",
					mock.Anything,
					mock.MatchedBy(func(filter *domain.TransactionFilter) bool {
						return filter.ReceiverID == validReceiverID &&
							filter.Limit == 20 &&
							filter.Offset == 5
					}),
					validUserID,
					"test-key-456",
				).Return(expectedTransactions, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:        "фильтрация с min_amount и max_amount",
			queryParams: "min_amount=100.00&max_amount=1000.00&limit=15&offset=0",
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-789",
			},
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
					validUserID,
					"test-key-789",
				).Return(expectedTransactions, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:        "невалидный JSON body",
			queryParams: "limit=10&offset=0",
			requestBody: `{"invalid json`,
			setupMock: func(m *mocks.TransactionUsecase) {
				// не вызываем GetTransactionFilter, т.к. ошибка парсинга JSON
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:        "невалидный limit параметр",
			queryParams: "limit=invalid&offset=0",
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-999",
			},
			setupMock: func(m *mocks.TransactionUsecase) {
				// не вызываем GetTransactionFilter, т.к. ошибка парсинга limit
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:        "невалидный UUID в sender_id",
			queryParams: "sender_id=invalid-uuid&limit=10&offset=0",
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-888",
			},
			setupMock: func(m *mocks.TransactionUsecase) {
				// не вызываем GetTransactionFilter, т.к. ошибка парсинга UUID
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:        "ошибка от usecase",
			queryParams: "sender_id=" + validSenderID.String() + "&limit=10&offset=0",
			requestBody: transactionDTO{
				UserID:         validUserID,
				IdempotencyKEY: "test-key-777",
			},
			setupMock: func(m *mocks.TransactionUsecase) {
				m.On("GetTransactionFilter",
					mock.Anything,
					mock.Anything,
					validUserID,
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
			handler := NewHandler(mockUsecase, mockAccountUsecase, mockAuthUsecae, serlog)

			tt.setupMock(mockUsecase)

			var bodyBytes []byte
			var err error
			if strBody, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(strBody)
			} else {
				bodyBytes, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodGet, "/transactions?"+tt.queryParams, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.Background())

			rr := httptest.NewRecorder()
			handler.TransactionFilter(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "неожиданный статус код")
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			if tt.expectError {
				assert.NotEmpty(t, rr.Body.String(), "ожидался response body с ошибкой")
			} else {
				// проверяем что ответ содержит валидный JSON с массивом транзакций
				var response []domain.Transaction
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				assert.NoError(t, err, "ответ должен быть валидным JSON")
			}
		})
	}
}
