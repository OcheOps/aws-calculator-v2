package config

import (
	"fmt"
	"os"
)

type AWSConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	PricingRegion   string
}

type Config struct {
	ListenAddr  string
	DatabaseURL string
	LogLevel    string
	Env         string
	AdminToken  string
	AWS         AWSConfig
	IngestCron  string
	FXAPIURL    string
	FXCron      string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:  getenv("API_LISTEN_ADDR", ":8080"),
		DatabaseURL: os.Getenv("API_DATABASE_URL"),
		LogLevel:    getenv("API_LOG_LEVEL", "info"),
		Env:         getenv("API_ENV", "development"),
		AdminToken:  os.Getenv("ADMIN_TOKEN"),
		AWS: AWSConfig{
			AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			PricingRegion:   getenv("AWS_PRICING_REGION", "us-east-1"),
		},
		IngestCron: getenv("INGEST_CRON", "0 3 * * *"),
		FXAPIURL:   getenv("FX_API_URL", "https://api.exchangerate.host"),
		FXCron:     getenv("FX_CRON", "@hourly"),
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
