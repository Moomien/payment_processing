package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"processing/internal/decimal"
	"processing/internal/delivery/http/mocks"
	"processing/internal/delivery/http/requestctx"
	"processing/internal/domain"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetAccountHandler(t *testing.T) {
	testUserID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	tests := []struct {
		name               string
		accountID          string
		setupMock          func(*mocks.AccountsUsecase)
		expectedStatusCode int
		expectError        bool
	}{
		{
			name:      "успешное получение аккаунта",
			accountID: testUserID.String(),
			setupMock: func(m *mocks.AccountsUsecase) {
				balance, _ := decimal.NewFromString("1000.50")
				expectedAccount := &domain.Account{
					ID:           testUserID,
					Name:         "Test User",
					Email:        "test@example.com",
					Balance:      balance,
					PasswordHash: "hashed_password",
					Role:         "user",
				}
				m.On("GetAccount",
					mock.Anything,
					testUserID,
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
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:      "доступ запрещен к чужому аккаунту",
			accountID: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001").String(),
			setupMock: func(m *mocks.AccountsUsecase) {
			},
			expectedStatusCode: http.StatusForbidden,
			expectError:        true,
		},
		{
			name:      "аккаунт не найден",
			accountID: testUserID.String(),
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("GetAccount",
					mock.Anything,
					testUserID,
				).Return(nil, errors.New("аккаунт не найден")).Once()
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectError:        true,
		},
		{
			name:      "ошибка БД при получении аккаунта",
			accountID: testUserID.String(),
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("GetAccount",
					mock.Anything,
					testUserID,
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
			handler := NewHandler(mockTransactionUsecase, mockAccountUsecase, mockAuthUsecase, nil)

			tt.setupMock(mockAccountUsecase)

			req := httptest.NewRequest(http.MethodGet, "/accounts/"+tt.accountID, nil)
			req.SetPathValue("id", tt.accountID)
			ctx := requestctx.WithIdentity(context.Background(), requestctx.Identity{UserID: testUserID.String()})
			req = req.WithContext(ctx)

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
				assert.Equal(t, testUserID, response.ID)
				assert.NotEmpty(t, response.Name)
				assert.NotEmpty(t, response.Email)
			}
		})
		t.Log("\n\n\n")
	}
}

func TestAccountTransactionsHandler(t *testing.T) {
	testUserID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
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
			accountID: testUserID.String(),
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
				amount1, _ := decimal.NewFromString("500.00")
				amount2, _ := decimal.NewFromString("250.50")
				expectedTransactions := []domain.Transaction{
					{
						ID:          validTransactionID1,
						Amount:      amount1,
						Sender_id:   testUserID,
						Receiver_id: uuid.MustParse("423e4567-e89b-12d3-a456-426614174003"),
						Status:      domain.StatusCompleted,
						Created_at:  time.Now(),
					},
					{
						ID:          validTransactionID2,
						Amount:      amount2,
						Sender_id:   uuid.MustParse("523e4567-e89b-12d3-a456-426614174004"),
						Receiver_id: testUserID,
						Status:      domain.StatusCompleted,
						Created_at:  time.Now().Add(-24 * time.Hour),
					},
				}
				m.On("TransactionHistory",
					mock.Anything,
					testUserID,
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
			accountID: testUserID.String(),
			limit:     "5",
			offset:    "10",
			setupMock: func(m *mocks.AccountsUsecase) {
				amount, _ := decimal.NewFromString("100.00")
				expectedTransactions := []domain.Transaction{
					{
						ID:          validTransactionID1,
						Amount:      amount,
						Sender_id:   testUserID,
						Receiver_id: uuid.MustParse("623e4567-e89b-12d3-a456-426614174005"),
						Status:      domain.StatusCompleted,
						Created_at:  time.Now(),
					},
				}
				m.On("TransactionHistory",
					mock.Anything,
					testUserID,
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
			accountID: testUserID.String(),
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("TransactionHistory",
					mock.Anything,
					testUserID,
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
			name:      "доступ запрещен к чужой истории",
			accountID: uuid.MustParse("123e4567-e89b-12d3-a456-426614174001").String(),
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
			},
			expectedStatusCode: http.StatusForbidden,
			expectError:        true,
		},
		{
			name:      "невалидный UUID аккаунта",
			accountID: "invalid-uuid",
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
			},
			expectedStatusCode: http.StatusBadRequest,
			expectError:        true,
		},
		{
			name:      "невалидный limit параметр",
			accountID: testUserID.String(),
			limit:     "invalid",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("TransactionHistory",
					mock.Anything,
					testUserID,
					10,
					0,
				).Return(25, []domain.Transaction{}, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
			expectedTotal:      25,
			expectedCount:      0,
		},
		{
			name:      "невалидный offset параметр",
			accountID: testUserID.String(),
			limit:     "10",
			offset:    "invalid",
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("TransactionHistory",
					mock.Anything,
					testUserID,
					10,
					0,
				).Return(25, []domain.Transaction{}, nil).Once()
			},
			expectedStatusCode: http.StatusOK,
			expectError:        false,
			expectedTotal:      25,
			expectedCount:      0,
		},
		{
			name:      "ошибка от usecase",
			accountID: testUserID.String(),
			limit:     "10",
			offset:    "0",
			setupMock: func(m *mocks.AccountsUsecase) {
				m.On("TransactionHistory",
					mock.Anything,
					testUserID,
					10,
					0,
				).Return(0, nil, errors.New("database error")).Once()
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
			handler := NewHandler(mockTransactionUsecase, mockAccountUsecase, mockAuthUsecase, nil)

			tt.setupMock(mockAccountUsecase)

			url := "/accounts/" + tt.accountID + "/transactions?limit=" + tt.limit + "&offset=" + tt.offset
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetPathValue("id", tt.accountID)
			ctx := requestctx.WithIdentity(context.Background(), requestctx.Identity{UserID: testUserID.String()})
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler.AccountTransactions(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code, "неожиданный статус код")

			if !tt.expectError {
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
				var response AccountTransactions
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err, "ответ должен быть валидным JSON")
				assert.Equal(t, tt.expectedTotal, response.Total, "неожиданное количество страниц")
				assert.Equal(t, tt.expectedCount, len(response.Transactions), "неожиданное количество транзакций")
			} else {
				assert.NotEmpty(t, rr.Body.String(), "ожидался response body с ошибкой")
			}
		})
		t.Log("\n\n\n")
	}
}
