package domain

import "errors"

var (
	ErrAccessDenied            = errors.New("доступ запрещен")
	ErrInsufficientFunds       = errors.New("недостаточно средств")
	ErrInvalidAmount           = errors.New("сумма должна быть положительной")
	ErrSameAccount             = errors.New("отправитель и получатель должны быть разными")
	ErrReceiverAccountNotFound = errors.New("receiver аккаунт не найден")
	ErrAccountAlreadyExist     = errors.New("аккаунт уже существует")
)

var (
	ErrSaveRefreshToken     = errors.New("ошибка сохранения рефреш токена")
	ErrRefreshTokenNotFound = errors.New("refresh токен не найден")
	ErrUserNotFound         = errors.New("связаннй с токеном юзер не найден")
	ErrRefreshTokenRevoked  = errors.New("refresh токен отозван")
	ErrRefreshTokenExpired  = errors.New("refresh токен истек")
	ErrInvalidCredentials   = errors.New("неверные учетные данные")
	ErrInvalidRefreshToken  = errors.New("невалидный refresh токен")
)
