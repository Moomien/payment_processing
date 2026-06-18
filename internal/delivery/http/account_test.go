package handlers

import (
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

func TestGetAccountHandler(t *testing.T) {
	service, err := os.OpenFile("service.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer service.Close()

	serlog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, service), nil))

	validAccountID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	tests := []struct {
		name               string
		accountID          string
		setupMock          func(*mocks.AccountsUsecase)
		expectedStatusCode int
		expectError        bool
	}{
		{
			name:      "успешное получение аккаунта",
			accountID: validAccountID.String(),
			setupMock: func(m *mocks.AccountsUsecase) {
				balance, _ := decimal.NewFromString("1000.50")
				expectedAccount := &domain.Account{
					ID:           validAccountID,
					Name:         "Test User",
					Email:        "test@example.com",
					Balance:      balance,
					PasswordHash: "hashed_password",
					Role:         "user",
				}
				m.On("GetAccount",
					mock.Anything,
					validAccountID,
				).Return(expectedAccount, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
		},
		{
			name:      "невалидный UUID аккаунта",
			accountID: "invalid-uuid",
			setupMock: func(m *mocks.AccountsUsecase) {
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:      "аккаунт не найден",
			accountID: validAccountID.String(),
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("GetAccount",
					mock.Anything,
					validAccountID,
				).Return(nil, errors.New("аккаунт не найден")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:      "ошибка БД при получении аккаунта",
			accountID: validAccountID.String(),
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("GetAccount",
					mock.Anything,
					validAccountID,
				).Return(nil, errors.New("database connection error")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAccountUsecase := mocks.NewAccountsUsecase(t)
			mockTransactionUsecase := mocks.NewTransactionUsecase(t)
			mockAuthUsecase := mocks.NewAuthUseCase(t)
			handler := NewHandler(mockTransactionUsecase, mockAccountUsecase, mockAuthUsecase, serlog)

			tt.setupMock(mockAccountUsecase)

			req := httptest.NewRequest(http.MethodGet, "/accounts/"+tt.accountID+"?id="+tt.accountID, nil)
			req = req.WithContext(context.Background())

			rr := httptest.NewRecorder()
			handler.GetAccount(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "неожиданный статус код")
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			if tt.expectError {
				assert.NotEmpty(t, rr.Body.String(), "ожидался response body с ошибкой")
			} else {
				var response domain.Account
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				assert.NoError(t, err, "ответ должен быть валидным JSON")
				assert.Equal(t, validAccountID, response.ID)
				assert.NotEmpty(t, response.Name)
				assert.NotEmpty(t, response.Email)
			}
		})
		t.Log("\n\n\n")
	}
}

func TestAccountTransactionsHandler(t *testing.T) {
	service, err := os.OpenFile("service.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer service.Close()

	serlog := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, service), nil))

	validAccountID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	validTransactionID1 := uuid.MustParse("223e4567-e89b-12d3-a456-426614174001")
	validTransactionID2 := uuid.MustParse("323e4567-e89b-12d3-a456-426614174002")

	tests := []struct {
		name               string
		accountID          string
		limit              string
		offset             string
		setupMock          func(*mocks.AccountsUsecase)
		expectedStatusCode int
		expectError        bool
		expectedTotal      int
		expectedCount      int
	}{
		{
			name:      "успешное получение истории транзакций",
			accountID: validAccountID.String(),
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
				amount1, _ := decimal.NewFromString("500.00")
				amount2, _ := decimal.NewFromString("250.50")
				expectedTransactions := []domain.Transaction{
					{
						ID:          validTransactionID1,
						Amount:      amount1,
						Sender_id:   validAccountID,
						Receiver_id: uuid.MustParse("423e4567-e89b-12d3-a456-426614174003"),
						Status:      domain.StatusCompleted,
						Created_at:  time.Now(),
					},
					{
						ID:          validTransactionID2,
						Amount:      amount2,
						Sender_id:   uuid.MustParse("523e4567-e89b-12d3-a456-426614174004"),
						Receiver_id: validAccountID,
						Status:      domain.StatusCompleted,
						Created_at:  time.Now().Add(-24 * time.Hour),
					},
				}
				m.On("TransactionHistory",
					mock.Anything,
					validAccountID,
					10,
					0,
				).Return(25, expectedTransactions, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
			expectedTotal:      25,
			expectedCount:      2,
		},
		{
			name:      "получение с пагинацией offset",
			accountID: validAccountID.String(),
			limit:     "5",
			offset:    "10",
			setupMock: func(m *mocks.AccountsUsecase) {
				amount, _ := decimal.NewFromString("100.00")
				expectedTransactions := []domain.Transaction{
					{
						ID:          validTransactionID1,
						Amount:      amount,
						Sender_id:   validAccountID,
						Receiver_id: uuid.MustParse("623e4567-e89b-12d3-a456-426614174005"),
						Status:      domain.StatusCompleted,
						Created_at:  time.Now(),
					},
				}
				m.On("TransactionHistory",
					mock.Anything,
					validAccountID,
					5,
					10,
				).Return(25, expectedTransactions, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
			expectedTotal:      25,
			expectedCount:      1,
		},
		{
			name:      "пустая история транзакций",
			accountID: validAccountID.String(),
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("TransactionHistory",
					mock.Anything,
					validAccountID,
					10,
					0,
				).Return(0, []domain.Transaction{}, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
			expectedTotal:      0,
			expectedCount:      0,
		},
		{
			name:      "невалидный UUID аккаунта",
			accountID: "invalid-uuid",
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:      "невалидный limit параметр",
			accountID: validAccountID.String(),
			limit:     "invalid",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:      "невалидный offset параметр",
			accountID: validAccountID.String(),
			limit:     "10",
			offset:    "invalid",
			setupMock: func(m *mocks.AccountsUsecase) {
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:      "ошибка от usecase",
			accountID: validAccountID.String(),
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("TransactionHistory",
					mock.Anything,
					validAccountID,
					10,
					0,
				).Return(0, nil, errors.New("database error")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:      "аккаунт не найден",
			accountID: validAccountID.String(),
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("TransactionHistory",
					mock.Anything,
					validAccountID,
					10,
					0,
				).Return(0, nil, errors.New("аккаунт не найден")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAccountUsecase := mocks.NewAccountsUsecase(t)
			mockTransactionUsecase := mocks.NewTransactionUsecase(t)
			mockAuthUsecase := mocks.NewAuthUseCase(t)
			handler := NewHandler(mockTransactionUsecase, mockAccountUsecase, mockAuthUsecase, serlog)

			tt.setupMock(mockAccountUsecase)

			url := "/accounts/" + tt.accountID + "/transactions?id=" + tt.accountID + "&limit=" + tt.limit + "&offset=" + tt.offset
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req = req.WithContext(context.Background())

			rr := httptest.NewRecorder()
			handler.AccountTransactions(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "неожиданный статус код")

			if !tt.expectError {
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
				var response AccountTransactions
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err, "ответ должен быть валидным JSON")
				assert.Equal(t, tt.expectedTotal, response.Total, "неожиданное количество страниц")
				assert.Equal(t, tt.expectedCount, len(response.Slice), "неожиданное количество транзакций")
			} else {
				assert.NotEmpty(t, rr.Body.String(), "ожидался response body с ошибкой")
			}
		})
		t.Log("\n\n\n")
	}
}
