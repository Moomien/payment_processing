package usecase

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"processing/internal/decimal"
	"processing/internal/domain"
	"processing/internal/infrastructure/logger"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost        = 12
	minPasswordLen    = 8
	maxPasswordLen    = 72
	maxEmailLen       = 254
	minAccountNameLen = 3
	maxAccountNameLen = 100
	dummyHash         = "$2a$12$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
)

var emailPattern = regexp.MustCompile(`^[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

type tokenManager interface {
	GenerateTokenPair(userID string, role string) (*domain.TokenPair, error)
	ValidateRefreshToken(token string) (*domain.RefreshClaims, error)
}

type AuthService struct {
	tx    domain.TxUOW
	cache domain.Cache
	jwt   tokenManager
	log   *slog.Logger
	now   func() time.Time
}

func NewAuthService(tx domain.TxUOW, cache domain.Cache, log *slog.Logger, jwt tokenManager) *AuthService {
	return &AuthService{
		tx:    tx,
		cache: cache,
		jwt:   jwt,
		log:   logger.WithService(log, "Auth"),
		now:   time.Now,
	}
}

func (as *AuthService) Register(ctx context.Context, email, password, name, ip string) (*domain.Account, error) {
	email = normalizeEmail(email)
	name = strings.TrimSpace(name)
	if err := validateRegistration(email, password, name); err != nil {
		return nil, err
	}
	if err := as.checkRateLimit(ctx, "register", ip); err != nil {
		return nil, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("хеширование пароля: %w", err)
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return nil, fmt.Errorf("открытие транзакции регистрации: %w", err)
	}
	defer func() { _ = uow.Rollback() }()

	account := &domain.Account{
		ID:           uuid.New(),
		Email:        email,
		Name:         name,
		PasswordHash: string(passwordHash),
		Balance:      decimal.Zero(),
		Role:         domain.RoleUser,
	}
	if err := uow.Accounts().Create(ctx, account); err != nil {
		return nil, fmt.Errorf("создание аккаунта: %w", err)
	}
	if err := uow.Commit(); err != nil {
		return nil, fmt.Errorf("коммит регистрации: %w", err)
	}

	as.log.InfoContext(ctx, "пользователь зарегистрирован", "user_id", account.ID)
	return account, nil
}

func (as *AuthService) Login(ctx context.Context, email, password, ip string) (*domain.TokenPair, error) {
	email = normalizeEmail(email)
	if err := as.checkRateLimit(ctx, "login", loginRateIdentity(ip, email)); err != nil {
		return nil, err
	}

	if !validEmail(email) || len(password) < minPasswordLen || len(password) > maxPasswordLen {
		compareDummyPassword(password)
		return nil, domain.ErrInvalidCredentials
	}

	account, err := as.loadAccountForLogin(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			compareDummyPassword(password)
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		as.log.WarnContext(ctx, "неверные учетные данные", "user_id", account.ID)
		return nil, domain.ErrInvalidCredentials
	}

	pair, err := as.jwt.GenerateTokenPair(account.ID.String(), account.Role)
	if err != nil {
		return nil, fmt.Errorf("генерация токенов: %w", err)
	}
	if err := as.saveRefreshSession(ctx, pair, account.ID); err != nil {
		return nil, err
	}

	as.log.InfoContext(ctx, "успешный вход", "user_id", account.ID)
	return publicTokenPair(pair), nil
}

func (as *AuthService) Refresh(ctx context.Context, refreshToken, ip string) (*domain.TokenPair, error) {
	if refreshToken == "" {
		return nil, domain.ErrInvalidRefreshToken
	}
	if err := as.checkRateLimit(ctx, "refresh", ip); err != nil {
		return nil, err
	}

	claims, err := as.jwt.ValidateRefreshToken(refreshToken)
	if err != nil {
		if errors.Is(err, domain.ErrTokenExpired) {
			return nil, domain.ErrRefreshTokenExpired
		}
		return nil, domain.ErrInvalidRefreshToken
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return nil, fmt.Errorf("открытие транзакции refresh: %w", err)
	}
	defer func() { _ = uow.Rollback() }()

	session, err := uow.Tokens().GetRefreshToken(ctx, claims.ID)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return nil, domain.ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("получение refresh-сессии: %w", err)
	}
	if claims.UserID != session.UserID.String() {
		return nil, domain.ErrInvalidRefreshToken
	}
	if session.Revoked {
		return nil, as.revokeAllAfterReuse(ctx, uow, session.UserID)
	}
	if !as.now().Before(session.ExpiresAt) {
		if err := uow.Tokens().RevokeRefreshToken(ctx, claims.ID); err != nil && !errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return nil, fmt.Errorf("отзыв истекшей refresh-сессии: %w", err)
		}
		if err := uow.Commit(); err != nil {
			return nil, fmt.Errorf("коммит отзыва истекшей refresh-сессии: %w", err)
		}
		return nil, domain.ErrRefreshTokenExpired
	}

	if err := uow.Tokens().RevokeRefreshToken(ctx, claims.ID); err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return nil, as.revokeAllAfterReuse(ctx, uow, session.UserID)
		}
		return nil, fmt.Errorf("отзыв refresh-сессии: %w", err)
	}

	account, err := uow.Accounts().GetById(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("получение аккаунта при refresh: %w", err)
	}
	newPair, err := as.jwt.GenerateTokenPair(account.ID.String(), account.Role)
	if err != nil {
		return nil, fmt.Errorf("генерация новых токенов: %w", err)
	}
	if err := uow.Tokens().SaveRefreshToken(ctx, newPair.JTI, account.ID.String(), newPair.ExpiresAt); err != nil {
		return nil, fmt.Errorf("сохранение новой refresh-сессии: %w", err)
	}
	if err := uow.Commit(); err != nil {
		return nil, fmt.Errorf("коммит rotation refresh-сессии: %w", err)
	}

	as.log.InfoContext(ctx, "refresh-сессия обновлена", "user_id", account.ID, "old_jti", claims.ID, "new_jti", newPair.JTI)
	return publicTokenPair(newPair), nil
}

// Logout is fail-open for cache/rate-limit failures: revocation must remain available.
func (as *AuthService) Logout(ctx context.Context, refreshToken, ip string) error {
	if err := as.checkRateLimit(ctx, "logout", ip); err != nil {
		as.log.WarnContext(ctx, "rate limit не блокирует logout", "err", err)
	}

	claims, err := as.jwt.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil
	}
	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return fmt.Errorf("открытие транзакции logout: %w", err)
	}
	defer func() { _ = uow.Rollback() }()

	session, err := uow.Tokens().GetRefreshToken(ctx, claims.ID)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return nil
		}
		return fmt.Errorf("получение refresh-сессии при logout: %w", err)
	}
	if claims.UserID != session.UserID.String() || session.Revoked {
		return nil
	}
	if err := uow.Tokens().RevokeRefreshToken(ctx, claims.ID); err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return nil
		}
		return fmt.Errorf("отзыв refresh-сессии при logout: %w", err)
	}
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("коммит logout: %w", err)
	}
	return nil
}

func (as *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return fmt.Errorf("открытие транзакции logout-all: %w", err)
	}
	defer func() { _ = uow.Rollback() }()

	if err := uow.Tokens().RevokeAllUserTokens(ctx, userID); err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return fmt.Errorf("отзыв всех refresh-сесий: %w", err)
	}
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("коммит logout-all: %w", err)
	}
	return nil
}

func (as *AuthService) loadAccountForLogin(ctx context.Context, email string) (*domain.Account, error) {
	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return nil, fmt.Errorf("открытие транзакции login: %w", err)
	}
	defer func() { _ = uow.Rollback() }()

	account, err := uow.Accounts().GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if err := uow.Commit(); err != nil {
		return nil, fmt.Errorf("коммит чтения аккаунта: %w", err)
	}
	return account, nil
}

func (as *AuthService) saveRefreshSession(ctx context.Context, pair *domain.TokenPair, userID uuid.UUID) error {
	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return fmt.Errorf("открытие транзакции refresh-сессии: %w", err)
	}
	defer func() { _ = uow.Rollback() }()
	if err := uow.Tokens().SaveRefreshToken(ctx, pair.JTI, userID.String(), pair.ExpiresAt); err != nil {
		return fmt.Errorf("сохранение refresh-сессии: %w", err)
	}
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("коммит refresh-сессии: %w", err)
	}
	return nil
}

func (as *AuthService) revokeAllAfterReuse(ctx context.Context, uow domain.UnitOfWork, userID uuid.UUID) error {
	err := uow.Tokens().RevokeAllUserTokens(ctx, userID)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return fmt.Errorf("отзыв сессий после reuse: %w", err)
	}
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("коммит отзыва после reuse: %w", err)
	}
	return domain.ErrRefreshTokenReuse
}

func (as *AuthService) checkRateLimit(ctx context.Context, operation, identity string) error {
	if err := as.cache.CheckRateLimit(ctx, "auth:"+operation+":"+identity); err != nil {
		return fmt.Errorf("rate limit %s: %w", operation, err)
	}
	return nil
}

func validateRegistration(email, password, name string) error {
	if !validEmail(email) {
		return domain.ErrInvalidEmail
	}
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return fmt.Errorf("длина пароля должна быть от %d до %d байт: %w", minPasswordLen, maxPasswordLen, domain.ErrInvalidPassword)
	}
	nameLen := utf8.RuneCountInString(name)
	if nameLen < minAccountNameLen || nameLen > maxAccountNameLen {
		return fmt.Errorf("длина имени должна быть от %d до %d символов: %w", minAccountNameLen, maxAccountNameLen, domain.ErrInvalidName)
	}
	return nil
}

func validEmail(email string) bool {
	return email != "" && len(email) <= maxEmailLen && emailPattern.MatchString(email)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func compareDummyPassword(password string) {
	if len(password) > maxPasswordLen {
		password = password[:maxPasswordLen]
	}
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
}

func loginRateIdentity(ip, email string) string {
	digest := sha256.Sum256([]byte(email))
	return fmt.Sprintf("%s:%x", ip, digest[:8])
}

func publicTokenPair(pair *domain.TokenPair) *domain.TokenPair {
	return &domain.TokenPair{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
	}
}
