package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"processing/internal/decimal"
	"processing/internal/domain"
	"time"

	"github.com/google/uuid"
)

type TransferService struct {
	tx    domain.TxUOW
	cache domain.Cache
	log   *slog.Logger
}

func NewService(tx domain.TxUOW, cache domain.Cache, loggerPath string) *TransferService {
	file, err := os.OpenFile(loggerPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), nil))
	slog.SetDefault(logger)
	slog.Info("создан логгер")
	return &TransferService{
		tx:    tx,
		cache: cache,
		log:   logger,
	}
}

// Transfer - главная функция процессинга. Создает транзакцию.
// Как работает: вычет с балансов аккаунтов -> создание транзакции
// принимает контекст, ключ для redis, sender_id, receiver_id, amount
func (ts *TransferService) Transfer(
	ctx context.Context,
	sender_id, receiver_id uuid.UUID,
	key string,
	amount decimal.Decimal,
) error {
	if err := ts.cache.CheckRateLimit(ctx, sender_id); err != nil {
		ts.log.Error("CheckRateLimit", "err", err)
		return err
	}
	//проверка идемпотентности запроса
	if err := ts.cache.IdempotencyCheck(ctx, key, 1, 24*time.Hour); err != nil {
		ts.log.Error("IdempotencyCheck", "err", err)
		return err
	}
	//новая транзакция
	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.Error("NewTX", "err", err)
		return err
	}

	defer uow.Rollback()

	//получаем пользователей по айди валидации
	sender, err := uow.Accounts().GetById(ctx, sender_id)
	if err != nil {
		ts.log.Error("Accounts.GetById", "err", err)
		return err
	}
	receiver, err := uow.Accounts().GetById(ctx, receiver_id)
	if err != nil {
		ts.log.Error("Account.GetById", "err", err)
		return err
	}
	//валидация
	if err := domain.ValidateTransferRequest(sender.ID, receiver.ID, sender.Balance, amount); err != nil {
		return err
	}
	//сначала вычитаем сумму с баланса отправителя
	if err := uow.Accounts().Sub(ctx, sender_id, amount); err != nil {
		ts.log.Error("DB substituion", "err", err)
		return err
	}
	//затем прибавляем сумму на баланс получателя
	if err := uow.Accounts().Add(ctx, receiver_id, amount); err != nil {
		ts.log.Error("DB Amount add", "err", err)
		return err
	}

	//создание транзакции
	tx, err := domain.NewTransaction(amount, sender_id, receiver_id)
	if err != nil {
		ts.log.Error("creating domain.Transaction", "err", err)
		return err
	}

	if err := uow.Transactions().Transaction(ctx, tx); err != nil {
		ts.log.Error("DB transaction creating", "err", err)
		return err
	}

	if err := uow.Transactions().UpdateStatus(ctx, tx, domain.StatusCompleted); err != nil {
		ts.log.Error("update status", "err", err)
		return err
	}

	return uow.Commit()
}

func (ts *TransferService) GetTransaction(
	ctx context.Context,
	transactionID,
	userID uuid.UUID,
	key string,
) (domain.Transaction, error) {
	if err := ts.cache.CheckRateLimit(ctx, userID); err != nil {
		ts.log.Error("CheckRateLimit", "err", err)
		return domain.Transaction{}, err
	}

	if err := ts.cache.IdempotencyCheck(ctx, key, 10, time.Minute); err != nil {
		ts.log.Error("IdempotencyCheck", "err", err)
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

	uow.Commit()
	return transaction, nil
}

func (ts *TransferService) GetTransactionFilter(
	ctx context.Context,
	t *domain.TransactionFilter,
	userID uuid.UUID,
	key string,
) ([]domain.Transaction, error) {
	if err := ts.cache.CheckRateLimit(ctx, userID); err != nil {
		ts.log.Error("CheckRateLimit", "err", err)
		return nil, err
	}

	if err := ts.cache.IdempotencyCheck(ctx, key, 10, time.Minute); err != nil {
		ts.log.Error("IdempotencyCheck", "err", err)
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
