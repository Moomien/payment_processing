package domain

import "errors"

var (
	ErrAccessDenied            = errors.New("доступ запрещен")
	ErrInsufficientFunds       = errors.New("недостаточно средств")
	ErrInvalidAmount           = errors.New("сумма должна быть положительной")
	ErrSameAccount             = errors.New("отправитель и получатель должны быть разными")
	ErrReceiverAccountNotFound = errors.New("receiver аккаунт не найден")
	ErrAccountNotFound         = errors.New("аккаунт не найден")
)

var (
	ErrSaveRefreshToken     = errors.New("ошибка сохранения рефреш токена")
	ErrRefreshTokenNotFound = errors.New("refresh токен не найден")
	ErrUserNotFound         = errors.New("связаннй с токеном юзер не найден")
	ErrRefreshTokenRevoked  = errors.New("refresh токен отозван")
	ErrRefreshTokenExpired  = errors.New("refresh токен истек")
	ErrInvalidCredentials   = errors.New("неверные учетные данные")
	ErrInvalidRefreshToken  = errors.New("невалидный refresh токен")
	ErrTokenExpired         = errors.New("токен истёк")
	ErrTokenInvalid         = errors.New("токен невалиден")
)

var (
	ErrAccountAlreadyExist = errors.New("аккаунт с таким email уже существует")
	ErrAccountBlocked      = errors.New("аккаунт заблокирован")
	ErrRateLimited         = errors.New("превышен лимит запросов")
	ErrDuplicateRequest    = errors.New("повторный запрос")
	ErrRefreshTokenReuse   = errors.New("повторное использование refresh токена")
	ErrInvalidEmail        = errors.New("невалидный email")
	ErrInvalidPassword     = errors.New("невалидный пароль")
	ErrInvalidName         = errors.New("невалидное имя")
)
