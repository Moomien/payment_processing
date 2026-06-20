package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment string
	LogLevel    string
	Postgres    PostgresConfig
	Redis       RedisConfig
}

type PostgresConfig struct {
	HOST     string
	PORT     string
	DBNAME   string
	USER     string
	PASSWORD string
	SSLMODE  string
}

type RedisConfig struct {
	HOST          string
	PORT          string
	USER          string
	PASSWORD      string
	RateLimitMin  int64
	RateLimitHour int64
	RateLimitDay  int64
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	cfg := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}

	cfg.Postgres = PostgresConfig{
		HOST:     getEnv("POSTGRES_HOST", "localhost"),
		PORT:     getEnv("POSTGRES_PORT", "5432"),
		DBNAME:   getEnv("POSTGRES_DB", "postgres_bd"),
		USER:     getEnv("POSTGRES_USER", "admin"),
		PASSWORD: getEnv("POSTGRES_PASSWORD", "secret"),
		SSLMODE:  getEnv("POSTGRES_SSL", "disable"),
	}

	cfg.Redis = RedisConfig{
		HOST:          getEnv("REDIS_HOST", "localhost"),
		PORT:          getEnv("REDIS_PORT", "6379"),
		USER:          getEnv("REDIS_USER", ""),
		PASSWORD:      getEnv("REDIS_PASSWORD", ""),
		RateLimitMin:  getEnvAsInt("REDIS_RATE_LIMIT_MIN", 20),
		RateLimitHour: getEnvAsInt("REDIS_RATE_LIMIT_HOUR", 100),
		RateLimitDay:  getEnvAsInt("REDIS_RATE_LIMIT_DAY", 500),
	}
	return cfg, nil
}

func (c *PostgresConfig) PostgresDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.USER, c.PASSWORD, c.HOST, c.PORT, c.DBNAME, c.SSLMODE,
	)
}

func (c *RedisConfig) RedisDSN() string {
	return fmt.Sprintf(
		"%s:%s@%s:%s",
		c.USER, c.PASSWORD, c.HOST, c.PORT,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}
