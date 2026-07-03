package config

import (
	"fmt"
	"os"
	"strconv"
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
	IngestCron   string
	FXAPIURL     string
	FXCron       string
	BaseURL      string
	CookieSecure bool
	CookieDomain string
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string
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
		FXAPIURL:     getenv("FX_API_URL", "https://api.exchangerate.host"),
		FXCron:       getenv("FX_CRON", "@hourly"),
		BaseURL:      getenv("PUBLIC_BASE_URL", "http://localhost"),
		CookieSecure: getenv("API_ENV", "development") == "production",
		CookieDomain: os.Getenv("COOKIE_DOMAIN"),
		SMTPHost:     getenv("SMTP_HOST", "smtp-relay.brevo.com"),
		SMTPPort:     getenvInt("SMTP_PORT", 587),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPass:     os.Getenv("SMTP_PASS"),
		SMTPFrom:     getenv("SMTP_FROM", "no-reply@example.com"),
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

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
