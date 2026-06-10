package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Cache - рейтлимитит запросы пользователя
// и кэширует запросы пользователей для последующей дедупликации
type Cache interface {
	CheckRateLimit(ctx context.Context, userID uuid.UUID) error
	IdempotencyCheck(ctx context.Context, key string, limit int64, TTL time.Duration) error
}
