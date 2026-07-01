package db

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOpen_InvalidURLFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := Open(ctx, "not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestOpen_RealDB(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
