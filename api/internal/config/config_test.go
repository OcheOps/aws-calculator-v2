package config

import (
	"testing"
)

func TestLoad_UsesDefaultsWhenEnvMissing(t *testing.T) {
	t.Setenv("API_LISTEN_ADDR", "")
	t.Setenv("API_LOG_LEVEL", "")
	t.Setenv("API_ENV", "")
	t.Setenv("API_DATABASE_URL", "postgres://x/y")
	t.Setenv("ADMIN_TOKEN", "t")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
}

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	t.Setenv("API_DATABASE_URL", "")
	t.Setenv("ADMIN_TOKEN", "t")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when API_DATABASE_URL missing")
	}
}

func TestLoad_RequiresAdminToken(t *testing.T) {
	t.Setenv("API_DATABASE_URL", "postgres://x/y")
	t.Setenv("ADMIN_TOKEN", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when ADMIN_TOKEN missing")
	}
}
