package db

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMigrate_AppliesInitialSchema(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()

	// Ensure a clean slate.
	_, _ = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)

	if err := Migrate(url); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM providers WHERE code = 'aws'`).Scan(&count); err != nil {
		t.Fatalf("query providers: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 aws provider row, got %d", count)
	}
}
