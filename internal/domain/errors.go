package domain

import "errors"

var (
	ErrInsufficientFunds = errors.New("недостаточно средств")
	ErrInvalidAmount     = errors.New("сумма должна быть положительной")
	ErrSameAccount       = errors.New("отправитель и получатель должны быть разными")
)
