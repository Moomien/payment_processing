package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrRateLimitExceed = errors.New("превышен лимит запросов")
	ErrDupRequest      = errors.New("запрос дубликат")
)

// Lua скрипт для sliding window rate limiting
// KEYS[1] - ключ для sorted set
// ARGV[1] - текущее время (timestamp)
// ARGV[2] - окно времени в секундах
// ARGV[3] - лимит запросов
// ARGV[4] - уникальный идентификатор запроса
var rateLimitScript = redis.NewScript(`
	local key = KEYS[1]
	local now = tonumber(ARGV[1])
	local window = tonumber(ARGV[2])
	local limit = tonumber(ARGV[3])
	local request_id = ARGV[4]
	
	local min_time = now - window
	redis.call('ZREMRANGEBYSCORE', key, '-inf', min_time)

	local current = redis.call('ZCARD', key)

	if current >= limit then
		return 0
	end
	redis.call('ZADD', key, now, request_id)
	
	redis.call('EXPIRE', key, window + 10)
	
	return 1
`)

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
func (redis *Redis) IdempotencyCheck(ctx context.Context, key string, TTL time.Duration) error {
	set, err := redis.client.SetNX(ctx, key, 1, TTL).Result()
	if err != nil {
		return err
	}

	if !set {
		return ErrDupRequest
	}
	return nil
}

// CheckRateLimit ограничивает запросы от пользователя
// принимает контекст и какой то id(user_id, ip, etc..)
func (redis *Redis) CheckRateLimit(ctx context.Context, id string) error {
	if err := redis.checkWindow(ctx, id, 5, time.Minute, "min"); err != nil {
		redis.log.InfoContext(ctx, "увеличение счетчика окна", "err", err)
		return err
	}

	if err := redis.checkWindow(ctx, id, 60, time.Hour, "hour"); err != nil {
		redis.log.InfoContext(ctx, "увеличение счетчика окна", "err", err)
		return err
	}

	if err := redis.checkWindow(ctx, id, 200, 24*time.Hour, "day"); err != nil {
		redis.log.InfoContext(ctx, "увеличение счетчика окна", "err", err)
		return err
	}

	return nil
}

func (redis *Redis) checkWindow(ctx context.Context, id string, limit int64, window time.Duration, suffix string) error {
	key := fmt.Sprintf("ratelimit:%s:%s", id, suffix)
	now := time.Now().Unix()
	windowSeconds := int64(window.Seconds())
	requestID := fmt.Sprintf("%d-%d", now, time.Now().UnixNano())

	result, err := rateLimitScript.Run(ctx, redis.client, []string{key}, now, windowSeconds, limit, requestID).Result()
	if err != nil {
		return fmt.Errorf("ошибка выполнения lua скрипта: %w", err)
	}

	allowed, ok := result.(int64)
	if !ok {
		return fmt.Errorf("неожиданный тип результата из lua скрипта")
	}

	if allowed == 0 {
		return ErrRateLimitExceed
	}

	return nil
}
