package domain

import (
	"processing/internal/decimal"

	"github.com/google/uuid"
)

// validateTransferRequest - валидирует реквест, проверяет достаточно ли денег на балансе сендера
// не является ли получатель отправителем, положительная ли сумма
func ValidateTransferRequest(
	sender uuid.UUID,
	receiver uuid.UUID,
	sender_balance decimal.Decimal,
	amount decimal.Decimal) error {
	if sender == receiver {
		return ErrSameAccount
	}

	if !amount.IsPositive() {
		return ErrInvalidAmount
	}

	if sender_balance.Compare(amount) == -1 {
		return ErrInsufficientFunds
	}
	return nil
}
