package config

import (
	"fmt"
	"os"

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
	HOST     string
	PORT     string
	USER     string
	PASSWORD string
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
		HOST:     getEnv("REDIS_HOST", "localhost"),
		PORT:     getEnv("REDIS_PORT", "6379"),
		USER:     getEnv("REDIS_USER", ""),
		PASSWORD: getEnv("REDIS_PASSWORD", ""),
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
		"redis://%s:%s@%s:%s",
		c.USER, c.PASSWORD, c.HOST, c.PORT,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
