package cache

import (
	"context"
	"errors"
	"fmt"
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
var rateLimitScript = redis.NewScript(
	`
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
`,
)

type Redis struct {
	client        *redis.Client
	rateLimitMin  int64
	rateLimitHour int64
	rateLimitDay  int64
}

type NewRedisOptions struct {
	Addr          string
	RateLimitMin  int64
	RateLimitHour int64
	RateLimitDay  int64
}

func NewRedis(opts NewRedisOptions) *Redis {
	c := redis.NewClient(&redis.Options{
		Addr: opts.Addr,
	})
	return &Redis{
		client:        c,
		rateLimitMin:  opts.RateLimitMin,
		rateLimitHour: opts.RateLimitHour,
		rateLimitDay:  opts.RateLimitDay,
	}
}

// IdempotencyCheck добавляет идемпотентности операции, проверяет не был ли уже такой запрос от ключа
// Атомарно устанавливает флаг на TTL. Повторный вызов с тем же ключом вернет ErrDupRequest
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
	if err := redis.checkWindow(ctx, id, redis.rateLimitMin, time.Minute, "min"); err != nil {
		return err
	}

	if err := redis.checkWindow(ctx, id, redis.rateLimitHour, time.Hour, "hour"); err != nil {
		return err
	}

	if err := redis.checkWindow(ctx, id, redis.rateLimitDay, 24*time.Hour, "day"); err != nil {
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
