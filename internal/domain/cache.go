package domain

import (
	"context"
	"time"
)

// Cache - рейтлимитит запросы пользователя
// и кэширует запросы пользователей для последующей дедупликации
type Cache interface {
	CheckRateLimit(ctx context.Context, id string) error
	IdempotencyCheck(ctx context.Context, key string, TTL time.Duration) error
}
