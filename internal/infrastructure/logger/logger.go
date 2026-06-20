package logger

import (
	"log/slog"
	"os"
)

func NewLogger(loglevel string, env string) (*slog.Logger, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(loglevel)); err != nil {
		level = slog.LevelInfo
	}

	var logger *slog.Logger
	switch env {
	case "production":
		logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		}))
	default:
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		}))
	}

	return logger, nil
}

func WithService(logger *slog.Logger, service string) *slog.Logger {
	return logger.With("service", service)
}
