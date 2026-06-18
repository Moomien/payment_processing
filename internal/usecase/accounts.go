package usecase

import (
	"context"
	"errors"
	"log/slog"
	"net/mail"
	"processing/internal/domain"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalivEmail = errors.New("invalid email")
)

type AccountsService struct {
	tx    domain.TxUOW
	cache domain.Cache
	log   *slog.Logger
}

func NewAccountService(tx domain.TxUOW, cache domain.Cache, log *slog.Logger) *AccountsService {
	return &AccountsService{
		tx:    tx,
		cache: cache,
		log:   log,
	}
}

// Create создает аккаунт
func (as *AccountsService) Create(ctx context.Context, acc *domain.Account, ip string) error {
	if err := as.cache.CheckRateLimit(ctx, ip); err != nil {
		return err
	}

	if err := as.cache.IdempotencyCheck(ctx, ip, time.Minute); err != nil {
		return err
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return err
	}
	defer uow.Rollback()

	mail, err := mail.ParseAddress(acc.Email)
	if err != nil {
		return err
	}

	if mail.Address == "" {
		return ErrInvalivEmail
	}

	if err := uow.Accounts().Create(ctx, acc); err != nil {
		return err
	}

	if err := uow.Commit(); err != nil {
		return err
	}
	return nil
}

// GetAccount получает аккаунт по id
func (as *AccountsService) GetAccount(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	if err := as.cache.CheckRateLimit(ctx, id.String()); err != nil {
		return nil, err
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return nil, err
	}
	defer uow.Rollback()

	acc, err := uow.Accounts().GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := uow.Commit(); err != nil {
		return nil, err
	}
	return acc, nil
}

// TransactionHistory выводит все транзакции пользователя
func (as *AccountsService) TransactionHistory(ctx context.Context, accountID uuid.UUID, limit, offset int) (int, []domain.Transaction, error) {
	if err := as.cache.CheckRateLimit(ctx, accountID.String()); err != nil {
		return 0, nil, err
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer uow.Rollback()

	var total int

	total, err = uow.Transactions().TotalTransactions(ctx, accountID)
	if err != nil {
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
		return 0, nil, err
	}

	if err := uow.Commit(); err != nil {
		return 0, nil, err
	}

	return total, transactions, nil
}
