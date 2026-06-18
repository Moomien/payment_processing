package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	jwtLayer "processing/internal/delivery/http/jwt"
	"processing/internal/domain"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	tx    domain.TxUOW
	cache domain.Cache
	log   *slog.Logger
}

func NewAuthService(tx domain.TxUOW, cache domain.Cache, log *slog.Logger) *AuthService {
	return &AuthService{
		tx:    tx,
		cache: cache,
		log:   log,
	}
}

// Register регистрирует нового пользователя
func (as *AuthService) Register(ctx context.Context, email, password, name string, ip string) (*domain.Account, error) {
	if err := as.cache.CheckRateLimit(ctx, ip); err != nil {
		as.log.WarnContext(ctx, "превышен лимит запросов при регистрации", "ip", ip)
		return nil, err
	}

	if err := as.cache.IdempotencyCheck(ctx, ip, time.Minute); err != nil {
		as.log.WarnContext(ctx, "повторный запрос на регистрацию", "ip", ip)
		return nil, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка хэширования пароля", "err", err)
		return nil, fmt.Errorf("хэширование пароля: %w", err)
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return nil, err
	}
	defer uow.Rollback()

	account := &domain.Account{
		ID:           uuid.New(),
		Email:        email,
		Name:         name,
		PasswordHash: string(passwordHash),
	}

	if err := uow.Accounts().Create(ctx, account); err != nil {
		as.log.ErrorContext(ctx, "ошибка создания аккаунта", "err", err, "email", email)
		return nil, err
	}

	if err := uow.Commit(); err != nil {
		return nil, err
	}

	as.log.InfoContext(ctx, "пользователь успешно зарегистрирован", "user_id", account.ID, "email", email)
	return account, nil
}

// Login аутентифицирует пользователя и возвращает пару токенов
func (as *AuthService) Login(ctx context.Context, email, password string, ip string) (*domain.TokenPair, error) {
	if err := as.cache.CheckRateLimit(ctx, ip); err != nil {
		as.log.WarnContext(ctx, "превышен лимит запросов при логине", "ip", ip)
		return nil, err
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return nil, err
	}
	defer uow.Rollback()

	account, err := uow.Accounts().GetByEmail(ctx, email)
	if err != nil {
		as.log.WarnContext(ctx, "попытка входа с несуществующим email", "email", email)
		return nil, domain.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		as.log.WarnContext(ctx, "неверный пароль при попытке входа", "user_id", account.ID, "email", email)
		return nil, domain.ErrInvalidCredentials
	}

	tokenPair, err := jwtLayer.GenerateTokenPair(account.ID.String(), account.Role)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка генерации токенов", "err", err, "user_id", account.ID)
		return nil, fmt.Errorf("генерация токенов: %w", err)
	}

	if err := uow.Tokens().SaveRefreshToken(ctx, tokenPair.JTI, account.ID.String(), tokenPair.ExpiresAt); err != nil {
		as.log.ErrorContext(ctx, "ошибка сохранения refresh токена", "err", err, "user_id", account.ID)
		return nil, err
	}

	if err := uow.Commit(); err != nil {
		return nil, err
	}

	as.log.InfoContext(ctx, "успешный вход пользователя", "user_id", account.ID, "email", email)

	return &domain.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// Refresh обновляет пару токенов используя refresh токен
func (as *AuthService) Refresh(ctx context.Context, refreshToken string, ip string) (*domain.TokenPair, error) {
	if err := as.cache.CheckRateLimit(ctx, ip); err != nil {
		as.log.WarnContext(ctx, "превышен лимит запросов при refresh", "ip", ip)
		return nil, err
	}

	claims, err := jwtLayer.ValidateRefreshToken(refreshToken)
	if err != nil {
		as.log.WarnContext(ctx, "невалидный refresh токен", "err", err)
		return nil, domain.ErrInvalidRefreshToken
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return nil, err
	}
	defer uow.Rollback()

	session, err := uow.Tokens().GetRefreshToken(ctx, claims.ID)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			as.log.WarnContext(ctx, "refresh токен не найден в БД", "jti", claims.ID)
			return nil, domain.ErrInvalidRefreshToken
		}
		return nil, err
	}

	if session.Revoked {
		as.log.WarnContext(ctx, "попытка использования отозванного токена", "jti", claims.ID, "user_id", session.UserID)
		return nil, domain.ErrRefreshTokenRevoked
	}

	if time.Now().After(session.ExpiresAt) {
		as.log.WarnContext(ctx, "истекший refresh токен", "jti", claims.ID, "user_id", session.UserID)
		return nil, domain.ErrRefreshTokenExpired
	}

	if err := uow.Tokens().RevokeRefreshToken(ctx, claims.ID); err != nil {
		as.log.ErrorContext(ctx, "ошибка отзыва старого токена", "err", err, "jti", claims.ID)
		return nil, err
	}

	account, err := uow.Accounts().GetById(ctx, session.UserID)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка получения аккаунта", "err", err, "user_id", session.UserID)
		return nil, err
	}

	newTokenPair, err := jwtLayer.GenerateTokenPair(account.ID.String(), account.Role)
	if err != nil {
		as.log.ErrorContext(ctx, "ошибка генерации новых токенов", "err", err, "user_id", account.ID)
		return nil, fmt.Errorf("генерация токенов: %w", err)
	}

	if err := uow.Tokens().SaveRefreshToken(ctx, newTokenPair.JTI, account.ID.String(), newTokenPair.ExpiresAt); err != nil {
		as.log.ErrorContext(ctx, "ошибка сохранения нового refresh токена", "err", err, "user_id", account.ID)
		return nil, err
	}

	if err := uow.Commit(); err != nil {
		return nil, err
	}

	as.log.InfoContext(ctx, "токены успешно обновлены", "user_id", account.ID, "old_jti", claims.ID, "new_jti", newTokenPair.JTI)

	return &domain.TokenPair{
		AccessToken:  newTokenPair.AccessToken,
		RefreshToken: newTokenPair.RefreshToken,
		ExpiresIn:    newTokenPair.ExpiresIn,
	}, nil
}

// Logout выходит из системы и отзывает refresh токен
func (as *AuthService) Logout(ctx context.Context, refreshToken string, ip string) error {
	if err := as.cache.CheckRateLimit(ctx, ip); err != nil {
		as.log.WarnContext(ctx, "превышен лимит запросов при logout", "ip", ip)
		return err
	}

	claims, err := jwtLayer.ValidateRefreshToken(refreshToken)
	if err != nil {
		as.log.WarnContext(ctx, "невалидный токен при logout", "err", err)
		return nil
	}

	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return err
	}
	defer uow.Rollback()

	if err := uow.Tokens().RevokeRefreshToken(ctx, claims.ID); err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			as.log.WarnContext(ctx, "токен уже удален или не существует", "jti", claims.ID)
			return nil
		}
		as.log.ErrorContext(ctx, "ошибка отзыва токена при logout", "err", err, "jti", claims.ID)
		return err
	}

	if err := uow.Commit(); err != nil {
		return err
	}

	as.log.InfoContext(ctx, "пользователь вышел из системы", "jti", claims.ID, "user_id", claims.UserID)
	return nil
}

// LogoutAll отзывает все токены
func (as *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	uow, err := as.tx.NewTX(ctx)
	if err != nil {
		return err
	}
	defer uow.Rollback()

	if err := uow.Tokens().RevokeAllUserTokens(ctx, userID); err != nil {
		as.log.ErrorContext(ctx, "ошибка отзыва всех токенов пользователя", "err", err, "user_id", userID)
		return err
	}

	if err := uow.Commit(); err != nil {
		return err
	}

	as.log.InfoContext(ctx, "все токены пользователя отозваны", "user_id", userID)
	return nil
}
