package domain

import "errors"

var (
	ErrInsufficientFunds       = errors.New("недостаточно средств")
	ErrInvalidAmount           = errors.New("сумма должна быть положительной")
	ErrSameAccount             = errors.New("отправитель и получатель должны быть разными")
	ErrReceiverAccountNotFound = errors.New("receiver аккаунт не найден")
)

var (
	ErrSaveRefreshToken     = errors.New("ошибка сохранения рефреш токена")
	ErrRefreshTokenNotFound = errors.New("refresh токен не найден")
	ErrRefreshTokenRevoked  = errors.New("refresh токен отозван")
	ErrRefreshTokenExpired  = errors.New("refresh токен истек")
	ErrInvalidCredentials   = errors.New("неверные учетные данные")
	ErrInvalidRefreshToken  = errors.New("невалидный refresh токен")
)
