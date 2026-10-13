package config

import (
	"log/slog"
	"strings"
	"testing"
)

func envFrom(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{"DATABASE_URL": "postgres://u:p@h:5432/d"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://u:p@h:5432/d" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.HTTPAddr != ":8000" {
		t.Errorf("HTTPAddr = %q, want :8000", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want INFO", cfg.LogLevel)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{
		"DATABASE_URL": "postgres://x",
		"HTTP_ADDR":    ":9000",
		"LOG_LEVEL":    "DEBUG",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9000" {
		t.Errorf("HTTPAddr = %q, want :9000", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want DEBUG", cfg.LogLevel)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	_, err := load(envFrom(map[string]string{}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v, want an error mentioning DATABASE_URL", err)
	}
}

func TestLoadRejectsUnknownLogLevel(t *testing.T) {
	_, err := load(envFrom(map[string]string{"DATABASE_URL": "postgres://x", "LOG_LEVEL": "verbose"}))
	if err == nil || !strings.Contains(err.Error(), "LOG_LEVEL") {
		t.Fatalf("err = %v, want an error mentioning LOG_LEVEL", err)
	}
}

func TestLoadDefaultsDataDir(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DataDir != "/data" {
		t.Errorf("DataDir = %q, want /data", cfg.DataDir)
	}
}

func TestLoadTrimsWhitespace(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{
		"DATABASE_URL": "  postgres://x  ",
		"HTTP_ADDR":    " :9000 ",
		"DATA_DIR":     " ../data/raw ",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://x" || cfg.HTTPAddr != ":9000" || cfg.DataDir != "../data/raw" {
		t.Errorf("cfg = %+v, want trimmed values", cfg)
	}
}

func TestLoadRejectsBlankDatabaseURL(t *testing.T) {
	_, err := load(envFrom(map[string]string{"DATABASE_URL": "   "}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v, want an error mentioning DATABASE_URL", err)
	}
}
