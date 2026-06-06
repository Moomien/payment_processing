package transfer

import (
	"context"
	"processing/internal/decimal"
	"processing/internal/storage"

	"github.com/google/uuid"
)

type TransferService struct {
	accounts      storage.AccountsStorage
	transasctions storage.TransactionStorage
}

func NewService(
	accounts storage.AccountsStorage,
	transactions storage.TransactionStorage) *TransferService {
	return &TransferService{accounts: accounts, transasctions: transactions}
}

func (ts *TransferService) Transfer(
	ctx context.Context, sender_id,
	receiver_id uuid.UUID,
	amount decimal.Decimal,
) error {
	//TODO: бизнес-логика процессинга. логика вычета с баланса аккаунта -> создание транзакции
}
