package usecase

import (
	"context"
	"log/slog"
	"processing/internal/decimal"
	"processing/internal/domain"
	"processing/internal/infrastructure/logger"
	"time"

	"github.com/google/uuid"
)

type TransactionsService struct {
	tx    domain.TxUOW
	cache domain.Cache
	log   *slog.Logger
}

func NewTransactionsService(txUOW domain.TxUOW, cache domain.Cache, log *slog.Logger) *TransactionsService {
	log = logger.WithService(log, "Transactions")
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
		ts.log.WarnContext(ctx, "превышен лимит запросов при переводе", "sender_id", sender_id)
		return "", err
	}

	if err := ts.cache.IdempotencyCheck(ctx, key, 24*time.Hour); err != nil {
		ts.log.WarnContext(ctx, "повторный запрос на перевод", "sender_id", sender_id, "receiver_id", receiver_id)
		return "", err
	}

	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка создания транзакции БД", "err", err)
		return "", err
	}

	defer uow.Rollback()

	sender, err := uow.Accounts().GetById(ctx, sender_id)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка получения аккаунта отправителя", "err", err, "sender_id", sender_id)
		return "", err
	}
	receiver, err := uow.Accounts().GetById(ctx, receiver_id)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка получения аккаунта получателя", "err", err, "receiver_id", receiver_id)
		return "", err
	}

	if sender.ID == receiver.ID {
		ts.log.WarnContext(ctx, "попытка перевода на собственный счет", "sender_id", sender_id)
		return "", domain.ErrSameAccount
	}

	if !amount.IsPositive() {
		ts.log.WarnContext(ctx, "попытка перевода отрицательной суммы", "sender_id", sender_id, "amount", amount)
		return "", domain.ErrInvalidAmount
	}

	if err := uow.Accounts().Sub(ctx, sender_id, amount); err != nil {
		ts.log.ErrorContext(ctx, "ошибка вычисления суммы со счета отправителя", "err", err, "sender_id", sender_id, "amount", amount)
		return "", err
	}

	if err := uow.Accounts().Add(ctx, receiver_id, amount); err != nil {
		ts.log.ErrorContext(ctx, "ошибка добавления суммы на счет получателя", "err", err, "receiver_id", receiver_id, "amount", amount)
		return "", err
	}

	tx, err := domain.NewTransaction(amount, sender_id, receiver_id)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка создания объекта транзакции", "err", err, "sender_id", sender_id, "receiver_id", receiver_id)
		return "", err
	}

	if err := uow.Transactions().Transaction(ctx, tx); err != nil {
		ts.log.ErrorContext(ctx, "ошибка сохранения транзакции в БД", "err", err, "transaction_id", tx.ID)
		return "", err
	}

	if err := uow.Transactions().UpdateStatus(ctx, tx, domain.StatusCompleted); err != nil {
		ts.log.ErrorContext(ctx, "ошибка обновления статуса транзакции", "err", err, "transaction_id", tx.ID)
		return "", err
	}

	if err := uow.Commit(); err != nil {
		ts.log.ErrorContext(ctx, "ошибка коммита транзакции БД", "err", err, "transaction_id", tx.ID)
		return "", err
	}

	ts.log.InfoContext(ctx, "транзакция успешно завершена", "transaction_id", tx.ID, "sender_id", sender_id, "receiver_id", receiver_id, "amount", amount)
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
		ts.log.WarnContext(ctx, "превышен лимит запросов при получении транзакции", "user_id", userID)
		return domain.Transaction{}, err
	}

	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка создания транзакции БД", "err", err)
		return domain.Transaction{}, err
	}
	defer uow.Rollback()

	transaction, err := uow.Transactions().GetByID(ctx, transactionID)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка получения транзакции из БД", "err", err, "transaction_id", transactionID)
		return domain.Transaction{}, err
	}

	if transaction.Sender_id != userID && transaction.Receiver_id != userID {
		ts.log.WarnContext(ctx, "попытка доступа к чужой транзакции", "user_id", userID, "transaction_id", transactionID)
		return domain.Transaction{}, domain.ErrAccessDenied
	}

	if err := uow.Commit(); err != nil {
		ts.log.ErrorContext(ctx, "ошибка коммита транзакции БД", "err", err)
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
		ts.log.WarnContext(ctx, "превышен лимит запросов при фильтрации транзакций", "user_id", userID)
		return nil, err
	}

	uow, err := ts.tx.NewTX(ctx)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка создания транзакции БД", "err", err)
		return nil, err
	}
	defer uow.Rollback()

	transactions, err := uow.Transactions().GetTransactions(ctx, *t)
	if err != nil {
		ts.log.ErrorContext(ctx, "ошибка получения транзакций из БД", "err", err, "account_id", t.AccountID)
		return nil, err
	}

	if err := uow.Commit(); err != nil {
		ts.log.ErrorContext(ctx, "ошибка коммита транзакции БД", "err", err)
		return nil, err
	}

	ts.log.InfoContext(ctx, "транзакции по фильтру получены", "user_id", userID, "count", len(transactions))
	return transactions, nil
}
