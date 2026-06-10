package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrRateLimitExceed = errors.New("превышен лимит запросов")
	ErrDupRequest      = errors.New("запрос дубликат")
)

type Redis struct {
	client *redis.Client
	log    *slog.Logger
}

func NewRedis(addr string, log *slog.Logger) *Redis {
	c := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &Redis{client: c, log: log}
}

// IdempotencyCheck - функция счётчик, проверяет не был ли уже такой запрос от пользователя
func (redis *Redis) IdempotencyCheck(ctx context.Context, key string, limit int64, TTL time.Duration) error {
	count, err := redis.client.Incr(ctx, key).Result()
	if err != nil {
		redis.log.InfoContext(ctx, "увеличение счетчика окна", "err", err)
		return err
	}

	if count == 1 {
		redis.client.Expire(ctx, key, TTL)
	}

	if count > limit {
		redis.log.InfoContext(ctx, "получен запрос дубликат", "err", err)
		return ErrDupRequest
	}

	return nil
}

// CheckRateLimit ограничивает запросы от пользователя
func (redis *Redis) CheckRateLimit(ctx context.Context, userID uuid.UUID) error {
	if err := redis.checkWindow(ctx, userID, 5, time.Minute, "min"); err != nil {
		redis.log.InfoContext(ctx, "увеличение счетчика окна", "err", err)
		return err
	}

	if err := redis.checkWindow(ctx, userID, 60, time.Hour, "hour"); err != nil {
		redis.log.InfoContext(ctx, "увеличение счетчика окна", "err", err)
		return err
	}

	if err := redis.checkWindow(ctx, userID, 200, 24*time.Hour, "day"); err != nil {
		redis.log.InfoContext(ctx, "увеличение счетчика окна", "err", err)
		return err
	}

	return nil
}

func (redis *Redis) checkWindow(ctx context.Context, userID uuid.UUID, limit int64, window time.Duration, suffix string) error {
	key := fmt.Sprintf("ratelimit:%s:%s", userID, suffix)

	count, err := redis.client.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	if count == 1 {
		redis.client.Expire(ctx, key, window)
	}

	if count > limit {
		return ErrRateLimitExceed
	}
	return nil
}
