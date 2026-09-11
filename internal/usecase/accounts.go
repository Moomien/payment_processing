package usecase

import (
	"context"
	"log/slog"
	"processing/internal/domain"
	"processing/internal/infrastructure/logger"

	"time"

	"github.com/google/uuid"
)

type AccountsService struct {
	tx    domain.TxUOW
	cache domain.Cache
	log   *slog.Logger
}

func NewAccountService(tx domain.TxUOW, cache domain.Cache, log *slog.Logger) *AccountsService {
	log = logger.WithService(log, "Accounts")
	return &AccountsService{
		tx:    tx,
		cache: cache,
		log:   log,
	}
}

// Create создает аккаунт
func (as *AccountsService) Create(ctx context.Context, acc *domain.Account, ip string) error {
	if err := as.cache.CheckRateLimit(ctx, ip); err != nil {
		as.log.WarnContext(ctx, "превышен лимит запросов при создании аккаунта", "ip", ip)
		return err
	}

	if err := as.cache.IdempotencyCheck(ctx, ip, time.Minute); err != nil {
		as.log.WarnContext(ctx, "повторный запрос на создание аккаунта", "ip", ip)
		return err
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка создания транзакции", "err", err)
		return err
	}
	defer uow.Rollback()

	if err := uow.Accounts().Create(ctx, acc); err != nil {
		as.log.ErrorContext(ctx, "ошибка создания аккаунта в БД", "err", err, "email", acc.Email)
		return err
	}

	if err := uow.Commit(); err != nil {
		as.log.ErrorContext(ctx, "ошибка коммита транзакции", "err", err)
		return err
	}

	as.log.InfoContext(ctx, "аккаунт успешно создан", "account_id", acc.ID, "email", acc.Email)
	return nil
}

// GetAccount получает аккаунт по id
func (as *AccountsService) GetAccount(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	if err := as.cache.CheckRateLimit(ctx, id.String()); err != nil {
		as.log.WarnContext(ctx, "превышен лимит запросов получения аккаунта", "account_id", id)
		return nil, err
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка создания транзакции", "err", err)
		return nil, err
	}
	defer uow.Rollback()

	acc, err := uow.Accounts().GetById(ctx, id)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка получения аккаунта из БД", "err", err, "account_id", id)
		return nil, err
	}

	if err := uow.Commit(); err != nil {
		as.log.ErrorContext(ctx, "ошибка коммита транзакции", "err", err)
		return nil, err
	}

	as.log.InfoContext(ctx, "аккаунт успешно получен", "account_id", id)
	return acc, nil
}

// TransactionHistory выводит все транзакции пользователя
func (as *AccountsService) TransactionHistory(ctx context.Context, accountID uuid.UUID, limit, offset int) (int, []domain.Transaction, error) {
	if err := as.cache.CheckRateLimit(ctx, accountID.String()); err != nil {
		as.log.WarnContext(ctx, "превышен лимит запросов истории транзакций", "account_id", accountID)
		return 0, nil, err
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка создания транзакции", "err", err)
		return 0, nil, err
	}
	defer uow.Rollback()

	var total int

	total, err = uow.Transactions().TotalTransactions(ctx, accountID)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка получения количества транзакций", "err", err, "account_id", accountID)
		return 0, nil, err
	}

	var transactions []domain.Transaction
	filter := domain.TransactionFilter{
		AccountID: accountID,
		Limit:     limit,
		Offset:    offset,
	}

	transactions, err = uow.Transactions().GetTransactions(ctx, filter)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка получения транзакций из БД", "err", err, "account_id", accountID, "limit", limit, "offset", offset)
		return 0, nil, err
	}

	if err := uow.Commit(); err != nil {
		as.log.ErrorContext(ctx, "ошибка коммита транзакции", "err", err)
		return 0, nil, err
	}

	as.log.InfoContext(ctx, "история транзакций получена", "account_id", accountID, "total", total, "returned", len(transactions))
	return total, transactions, nil
}
