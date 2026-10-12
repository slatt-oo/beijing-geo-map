package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	LogLevel    slog.Level
}

func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL: getenv("DATABASE_URL"),
		HTTPAddr:    getenv("HTTP_ADDR"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL 未设置")
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8000"
	}
	level, err := parseLevel(getenv("LOG_LEVEL"))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level
	return cfg, nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL 取值无效: %q（可选 debug/info/warn/error）", s)
	}
}
