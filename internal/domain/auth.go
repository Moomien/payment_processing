package domain

import (
	"context"

	"github.com/google/uuid"
)

type AuthUseCase interface {
	Register(ctx context.Context, email, password, name string, ip string) (*Account, error)
	Login(ctx context.Context, email, password string, ip string) (*TokenPair, error)
	Refresh(ctx context.Context, refreshToken string, ip string) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string, ip string) error
	LogoutAll(ctx context.Context, userID uuid.UUID) error
}
