package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

func TestIdempotencyCheck(t *testing.T) {
	mr := miniredis.RunT(t)

	client := NewRedis(mr.Addr())
	sender_id, _ := uuid.NewUUID()
	transaction_id, _ := uuid.NewUUID()
	if err := client.IdempotencyCheck(context.Background(), sender_id, transaction_id); err != nil {
		t.Log(err)
	}
	t.Log("запрос уникальный")
	if err := client.IdempotencyCheck(context.Background(), sender_id, transaction_id); err != nil {
		t.Log(err)
	}
}

func TestRedisMinutes(t *testing.T) {
	mr := miniredis.RunT(t)

	client := NewRedis(mr.Addr())
	userID, _ := uuid.NewUUID()
	for range 5 {
		if err := client.CheckRateLimit(context.Background(), userID); err != nil {
			t.Log(err)
		}
	}

	if err := client.CheckRateLimit(context.Background(), userID); err != nil {
		t.Log(err)
	}

	mr.FastForward(time.Minute)
	t.Log("промотали время вперед")
	if err := client.CheckRateLimit(context.Background(), userID); err != nil {
		t.Log(err)
	}
	t.Log("успех")
}

func TestRedisHours(t *testing.T) {
	mr := miniredis.RunT(t)
	client := NewRedis(mr.Addr())
	userID, _ := uuid.NewUUID()

	for range 60 {
		mr.FastForward(time.Minute)
		if err := client.CheckRateLimit(context.Background(), userID); err != nil {
			t.Log(err)
		}
	}
	t.Log("Отослали 60 запросов")

	t.Log("отслыаем еще один запрос")
	if err := client.CheckRateLimit(context.Background(), userID); err != nil {
		t.Log(err)
	}

	mr.FastForward(time.Hour)
	t.Log("промотали время на 1 час вперед")
	if err := client.CheckRateLimit(context.Background(), userID); err != nil {
		t.Log(err)
	}
	t.Log("успех")
}

func TestRedisDay(t *testing.T) {
	mr := miniredis.RunT(t)
	client := NewRedis(mr.Addr())
	userID, _ := uuid.NewUUID()

	for range 200 {
		mr.FastForward(time.Minute)
		if err := client.CheckRateLimit(context.Background(), userID); err != nil {
			t.Log(err)
		}
	}
	t.Log("Отослали 200 запросов")

	t.Log("отслыаем еще один запрос")
	if err := client.CheckRateLimit(context.Background(), userID); err != nil {
		t.Log(err)
	}

	mr.FastForward(time.Hour)
	t.Log("промотали время на 1 час вперед")
	if err := client.CheckRateLimit(context.Background(), userID); err != nil {
		t.Log(err)
	}

	mr.FastForward(24 * time.Hour)
	t.Log("промотали время на 24 часа вперед")
	if err := client.CheckRateLimit(context.Background(), userID); err != nil {
		t.Log(err)
	}
	t.Log("успех")
}
