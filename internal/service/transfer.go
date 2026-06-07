package service

import (
	"context"
	"errors"
	"fmt"
	"processing/internal/cache"
	"processing/internal/decimal"
	"processing/internal/storage"

	"github.com/google/uuid"
)

var (
	ErrInsufficientFunds = errors.New("недостаточно средств")
	ErrInvalidAmount     = errors.New("сумма должна быть положительной")
	ErrSameAccount       = errors.New("отправитель и получатель должны быть разными")
)

type TransferService struct {
	factory storage.UoWFactory
	cache   cache.Cache
}

func NewService(factory storage.UoWFactory, cache cache.Cache) *TransferService {
	return &TransferService{factory: factory, cache: cache}
}

// Transfer - главная функция процессинга. Создает транзакцию.
// Как работает: вычет с балансов аккаунтов -> создание транзакции
// принимает контекст, ключ для redis, sender_id, receiver_id, amount
func (ts *TransferService) Transfer(
	ctx context.Context,
	sender_id, receiver_id uuid.UUID,
	amount decimal.Decimal,
) error {
	if err := ts.cache.CheckRateLimit(ctx, sender_id); err != nil {
		return fmt.Errorf("ratelimit %s: %w", sender_id, err)
	}

	uow, err := ts.factory.NewUoW(ctx)
	if err != nil {
		return err
	}
	defer uow.Rollback()

	//получаем пользователей по айди валидации
	sender, err := uow.Accounts().GetById(ctx, sender_id)
	if err != nil {
		return err
	}
	receiver, err := uow.Accounts().GetById(ctx, receiver_id)
	if err != nil {
		return err
	}
	//валидация
	if err := validateTransferRequest(sender, receiver, amount); err != nil {
		return fmt.Errorf("валидация запроса: %w", err)
	}
	//сначала вычитаем сумму с баланса отправителя
	if err := uow.Accounts().Sub(ctx, sender_id, amount); err != nil {
		return err
	}
	//затем прибавляем сумму на баланс получателя
	if err := uow.Accounts().Add(ctx, receiver_id, amount); err != nil {
		return err
	}

	//создание транзакции
	tx, err := storage.NewTransaction(amount, sender_id, receiver_id)
	if err != nil {
		return err
	}
	//проверка идемпотентности запроса
	if err := ts.cache.IdempotencyCheck(ctx, sender_id, tx.ID); err != nil {
		return err
	}

	if err := uow.Transactions().Transaction(ctx, tx); err != nil {
		return err
	}

	if err := uow.Transactions().UpdateStatus(ctx, tx, storage.StatusCompleted); err != nil {
		return err
	}

	return uow.Commit()
}

// validateTransferRequest - валидирует реквест, проверяет достаточно ли денег на балансе сендера
// не является ли получатель отправителем, положительная ли сумма
func validateTransferRequest(
	sender *storage.Account,
	receiver *storage.Account,
	amount decimal.Decimal) error {
	if sender.ID == receiver.ID {
		return ErrSameAccount
	}

	if !amount.IsPositive() {
		return ErrInvalidAmount
	}

	if sender.Balance.Compare(amount) == -1 {
		return ErrInsufficientFunds
	}
	return nil
}
