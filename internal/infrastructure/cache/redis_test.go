package cache

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

func TestIdempotencyCheck(t *testing.T) {
	file, err := os.OpenFile("redis_test.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), nil))
	mr := miniredis.RunT(t)

	client := NewRedis(mr.Addr(), logger)
	key := "somekey"
	if err := client.IdempotencyCheck(context.Background(), key, 1, 24*time.Hour); err != nil {
		t.Log(err)
		return
	}
	t.Log("запрос уникальный")
	if err := client.IdempotencyCheck(context.Background(), key, 1, 24*time.Hour); err != nil {
		t.Log(err)
	}
}

func TestRedisMinutes(t *testing.T) {
	file, err := os.OpenFile("redis_test.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), nil))
	mr := miniredis.RunT(t)

	client := NewRedis(mr.Addr(), logger)
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
		return
	}
	t.Log("успех")
}

func TestRedisHours(t *testing.T) {
	file, err := os.OpenFile("redis_test.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), nil))
	mr := miniredis.RunT(t)
	client := NewRedis(mr.Addr(), logger)
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
		return
	}
	t.Log("успех")
}

func TestRedisDay(t *testing.T) {
	file, err := os.OpenFile("redis_test.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), nil))
	mr := miniredis.RunT(t)
	client := NewRedis(mr.Addr(), logger)
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
		return
	}
	t.Log("успех")
}
