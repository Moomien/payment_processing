package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"processing/internal/decimal"
	"processing/internal/domain"
	"time"

	"github.com/google/uuid"
)

type TransactionsService struct {
	tx    domain.TxUOW
	cache domain.Cache
	log   *slog.Logger
}

func NewTransactionsService(txUOW domain.TxUOW, cache domain.Cache, log *slog.Logger) *TransactionsService {
	return &TransactionsService{
		tx:    txUOW,
		cache: cache,
		log:   log,
	}
}

// Transfer - главная функция процессинга. Создает транзакцию.
// Как работает: вычет с балансов аккаунтов -> создание транзакции
// принимает контекст, sender_id, receiver_id, ключ для redis, amount
func (ts *TransactionsService) Transfer(
	ctx context.Context,
	sender_id, receiver_id uuid.UUID,
	key string,
	amount decimal.Decimal,
) (string, error) {
	if err := ts.cache.CheckRateLimit(ctx, sender_id.String()); err != nil {
		ts.log.Error("CheckRateLimit", "err", err)
		return "", err
	}

	if err := ts.cache.IdempotencyCheck(ctx, key, 24*time.Hour); err != nil {
		ts.log.Error("IdempotencyCheck", "err", err)
		return "", err
	}

	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.Error("NewTX", "err", err)
		return "", err
	}

	defer uow.Rollback()

	sender, err := uow.Accounts().GetById(ctx, sender_id)
	if err != nil {
		ts.log.Error("Accounts.GetById", "err", err)
		return "", err
	}
	receiver, err := uow.Accounts().GetById(ctx, receiver_id)
	if err != nil {
		ts.log.Error("Account.GetById", "err", err)
		return "", err
	}

	if sender.ID == receiver.ID {
		return "", domain.ErrSameAccount
	}

	if !amount.IsPositive() {
		return "", domain.ErrInvalidAmount
	}

	if err := uow.Accounts().Sub(ctx, sender_id, amount); err != nil {
		ts.log.Error("DB substituion", "err", err)
		return "", err
	}

	if err := uow.Accounts().Add(ctx, receiver_id, amount); err != nil {
		ts.log.Error("DB Amount add", "err", err)
		return "", err
	}

	tx, err := domain.NewTransaction(amount, sender_id, receiver_id)
	if err != nil {
		ts.log.Error("creating domain.Transaction", "err", err)
		return "", err
	}

	if err := uow.Transactions().Transaction(ctx, tx); err != nil {
		ts.log.Error("DB transaction creating", "err", err, "transaction ID", tx.ID)
		return "", err
	}

	if err := uow.Transactions().UpdateStatus(ctx, tx, domain.StatusCompleted); err != nil {
		ts.log.Error("update status", "err", err)
		return "", err
	}

	if err := uow.Commit(); err != nil {
		return "", err
	}

	return tx.ID.String(), nil
}

// GetTransaction
func (ts *TransactionsService) GetTransaction(
	ctx context.Context,
	transactionID,
	userID uuid.UUID,
	key string,
) (domain.Transaction, error) {
	if err := ts.cache.CheckRateLimit(ctx, userID.String()); err != nil {
		ts.log.Error("CheckRateLimit", "err", err)
		return domain.Transaction{}, err
	}

	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.Error("NewTX", "err", err)
		return domain.Transaction{}, fmt.Errorf("ошибка начала транзакции бд: %w", err)
	}
	defer uow.Rollback()

	transaction, err := uow.Transactions().GetByID(ctx, transactionID)
	if err != nil {
		ts.log.Error("Transactions.GetByID", "err", err)
		return domain.Transaction{}, fmt.Errorf("ошибка получения транзакции из бд: %w", err)
	}

	if transaction.Sender_id != userID && transaction.Receiver_id != userID {
		ts.log.WarnContext(ctx, "попытка доступа к чужой транзакции", "user_id", userID, "transaction_id", transactionID)
		return domain.Transaction{}, fmt.Errorf("доступ запрещен")
	}

	if err := uow.Commit(); err != nil {
		return domain.Transaction{}, err
	}
	return transaction, nil
}

func (ts *TransactionsService) GetTransactionFilter(
	ctx context.Context,
	t *domain.TransactionFilter,
	userID uuid.UUID,
	key string,
) ([]domain.Transaction, error) {
	if err := ts.cache.CheckRateLimit(ctx, userID.String()); err != nil {
		ts.log.Error("CheckRateLimit", "err", err)
		return nil, err
	}

	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.Error("NewTX", "err", err)
		return nil, fmt.Errorf("ошибка начала транзакции бд: %w", err)
	}
	defer uow.Rollback()

	transactions, err := uow.Transactions().GetTransactions(ctx, *t)
	if err != nil {
		ts.log.Error("Transactions.GetTransactions", "err", err)
		return nil, fmt.Errorf("ошибка получения транзакций из бд: %w", err)
	}

	if err := uow.Commit(); err != nil {
		ts.log.Error("Commit", "err", err)
		return nil, fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	return transactions, nil
}
