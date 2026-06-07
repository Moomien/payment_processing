package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrRateLimitExceed = errors.New("превышен лимит запросов")
	ErrDupRequest      = errors.New("запрос дубликат")
)

// Cache - рейтлимитит запросы пользователя
// и кэширует запросы пользователей для последующей дедупликации
type Cache interface {
	CheckRateLimit(ctx context.Context, userID uuid.UUID) error
	IdempotencyCheck(ctx context.Context, sender_id uuid.UUID, transaction_id uuid.UUID) error
}

type Redis struct {
	client *redis.Client
}

func NewRedis(addr string) *Redis {
	c := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &Redis{client: c}
}

// IdempotencyCheck - функция счётчик, проверяет не был ли уже такой запрос от пользователя
func (redis *Redis) IdempotencyCheck(ctx context.Context, sender_id uuid.UUID, transaction_id uuid.UUID) error {
	key := fmt.Sprintf("idempotency:key:%s:%s", sender_id, transaction_id)
	count, err := redis.client.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("увеличение счетчика окна: %w", err)
	}

	if count > 1 {
		return ErrDupRequest
	}

	return nil
}

// CheckRateLimit ограничивает запросы от пользователя
func (redis *Redis) CheckRateLimit(ctx context.Context, userID uuid.UUID) error {
	if err := redis.checkWindow(ctx, userID, 5, time.Minute, "min"); err != nil {
		return err
	}

	if err := redis.checkWindow(ctx, userID, 60, time.Hour, "hour"); err != nil {
		return err
	}

	if err := redis.checkWindow(ctx, userID, 200, 24*time.Hour, "day"); err != nil {
		return err
	}

	return nil
}

func (redis *Redis) checkWindow(ctx context.Context, userID uuid.UUID, limit int64, window time.Duration, suffix string) error {
	key := fmt.Sprintf("ratelimit:%s:%s", userID, suffix)

	count, err := redis.client.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("увеличение счетчика окна: %w", err)
	}

	if count == 1 {
		redis.client.Expire(ctx, key, window)
	}

	if count > limit {
		return ErrRateLimitExceed
	}
	return nil
}
