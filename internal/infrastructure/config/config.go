package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment string
	LogLevel    string
	HTTP        HTTPConfig
	Postgres    PostgresConfig
	Redis       RedisConfig
	JWT         JWTConfig
	Ratelimit   RateLimitConfig
}

type HTTPConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration // короткий TTL
	RefreshTTL    time.Duration // длинный TTL
}

type PostgresConfig struct {
	HOST     string
	PORT     string
	DBNAME   string
	USER     string
	PASSWORD string
	SSLMODE  string
	MaxConns int
}

type RedisConfig struct {
	HOST          string
	PORT          string
	USER          string
	PASSWORD      string
	DB            int
	RateLimitMin  int64
	RateLimitHour int64
	RateLimitDay  int64
}

type RateLimitConfig struct {
	PerMinute int64
	PerHour   int64
	PerDay    int64
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	cfg := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}

	cfg.HTTP = HTTPConfig{
		Port:            getEnv("HTTP_PORT", "8080"),
		ReadTimeout:     getEnvAsDuration("HTTP_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    getEnvAsDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		ShutdownTimeout: getEnvAsDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	cfg.Postgres = PostgresConfig{
		HOST:     getEnv("POSTGRES_HOST", "localhost"),
		PORT:     getEnv("POSTGRES_PORT", "5432"),
		DBNAME:   getEnv("POSTGRES_DB", ""),
		USER:     getEnv("POSTGRES_USER", ""),
		PASSWORD: getEnv("POSTGRES_PASSWORD", ""),
		SSLMODE:  getEnv("POSTGRES_SSLMODE", "require"),
		MaxConns: int(getEnvAsInt("POSTGRES_MAX_CONNS", 10)),
	}

	cfg.Redis = RedisConfig{
		HOST:     getEnv("REDIS_HOST", "localhost"),
		PORT:     getEnv("REDIS_PORT", "6379"),
		USER:     getEnv("REDIS_USER", ""),
		PASSWORD: getEnv("REDIS_PASSWORD", ""),
		DB:       int(getEnvAsInt("REDIS_DB", 0)),
	}

	cfg.Ratelimit = RateLimitConfig{
		PerMinute: getEnvAsInt("RATE_LIMIT_PER_MINUTE", 60),
		PerHour:   getEnvAsInt("RATE_LIMIT_PER_HOUR", 1000),
		PerDay:    getEnvAsInt("RATE_LIMIT_PER_DAY", 10000),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	var errs []error
	if !oneOf(c.Environment, "developmnet", "staging", "production") {
		errs = append(errs, fmt.Errorf("ENVIRONMENT: недопустимое значение %q", c.Environment))
	}

	if !oneOf(c.LogLevel, "debug", "info", "warn", "error") {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: недопустимое значение %q", c.LogLevel))
	}

	errs = append(errs, required("POSTGRES_HOST", c.Postgres.HOST))
	errs = append(errs, required("POSTGRES_PORT", c.Postgres.PORT))
	errs = append(errs, required("POSTGRES_DB", c.Postgres.DBNAME))
	errs = append(errs, required("POSTGRES_USER", c.Postgres.USER))
	errs = append(errs, required("POSTGRES_PASSWORD", c.Postgres.PASSWORD))

	if !oneOf(c.Postgres.SSLMODE, "disable", "allow", "prefer", "require", "verify-ca", "verify-full") {
		errs = append(errs, fmt.Errorf("POSTGRES_SSLMODE: недопустимое значение %q", c.Postgres.SSLMODE))
	}
	if c.Postgres.MaxConns <= 0 {
		errs = append(errs, errors.New("POSTGRES_MAX_CONNS: должен быть больше нуля"))
	}

	errs = append(errs, required("REDIS_HOST", c.Redis.HOST))
	errs = append(errs, required("REDIS_PORT", c.Redis.PORT))
	if c.Redis.DB < 0 {
		errs = append(errs, errors.New("REDIS_DB: не может быть отрицательным"))
	}

	if c.JWT.AccessSecret == "" || c.JWT.RefreshSecret == "" {
		errs = append(errs, errors.New("ACCESS_TOKEN_SECRET И REFRESH_TOKEN_SECRET не могут быть пустыми"))
	}
	if c.JWT.AccessSecret == c.JWT.RefreshSecret && c.JWT.AccessSecret != "" {
		errs = append(errs, errors.New("ACCESS_TOKEN_SECRET и REFRESH_TOKEN_SECRET должны различаться"))
	}
	if c.JWT.AccessTTL >= c.JWT.RefreshTTL {
		errs = append(errs, errors.New("ACCESS_TOKEN_TTL должен быть меньше REFRESH_TOKEN_TTL"))
	}

	if c.Ratelimit.PerMinute <= 0 || c.Ratelimit.PerHour <= 0 || c.Ratelimit.PerDay <= 0 {
		errs = append(errs, errors.New("RATE_LIMIT_*: все лимиты должны быть положительными"))
	}
	if c.Ratelimit.PerMinute > c.Ratelimit.PerHour || c.Ratelimit.PerHour > c.Ratelimit.PerDay {
		errs = append(errs, errors.New("RATE_LIMIT_*: лимиты должны возрастать(minute <= hour <= day)"))
	}

	if c.Environment == "production" {
		if c.Postgres.SSLMODE == "disable" || c.Postgres.SSLMODE == "allow" || c.Postgres.SSLMODE == "prefer" {
			errs = append(errs, fmt.Errorf("POSTGRES_SSLMODE=%q защищен в production", c.Postgres.SSLMODE))
		}
		if c.Redis.PASSWORD == "" {
			errs = append(errs, errors.New("REDIS_PASSWORD: обязателен в production"))
		}
	}

	return errors.Join(errs...)
}

func oneOf(value string, allowed ...string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	return false
}

func required(key, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s: обязательная переменная не задана", key)
	}
	return nil
}

func (c *PostgresConfig) PostgresDSN() string {
	q := url.Values{}
	q.Set("sslmode", c.SSLMODE)
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.USER, c.PASSWORD),
		Host:     net.JoinHostPort(c.HOST, c.PORT),
		Path:     "/" + c.DBNAME,
		RawQuery: q.Encode(),
	}
	return u.String()
}

func (c *RedisConfig) Addr() string {
	return net.JoinHostPort(c.HOST, c.PORT)
}

func getEnv(key, defaultValue string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return -1
	}
	return i
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return -1
	}
	return d
}
