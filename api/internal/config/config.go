package config

import (
	"fmt"
	"os"
)

type Config struct {
	ListenAddr  string
	DatabaseURL string
	LogLevel    string
	Env         string
	AdminToken  string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:  getenv("API_LISTEN_ADDR", ":8080"),
		DatabaseURL: os.Getenv("API_DATABASE_URL"),
		LogLevel:    getenv("API_LOG_LEVEL", "info"),
		Env:         getenv("API_ENV", "development"),
		AdminToken:  os.Getenv("ADMIN_TOKEN"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("API_DATABASE_URL is required")
	}
	if cfg.AdminToken == "" {
		return Config{}, fmt.Errorf("ADMIN_TOKEN is required")
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
