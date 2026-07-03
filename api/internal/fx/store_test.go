package fx

import (
	"context"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/shopspring/decimal"
)

func setupStore(t *testing.T) (*Store, context.Context) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(pool.Close)
	_, _ = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewStore(pool), ctx
}

func TestStore_UpsertAndLatest(t *testing.T) {
	s, ctx := setupStore(t)
	rates := []Rate{
		{Base: "USD", Quote: "EUR", Rate: decimal.NewFromFloat(0.92)},
		{Base: "USD", Quote: "NGN", Rate: decimal.NewFromFloat(1500)},
	}
	if err := s.Upsert(ctx, rates); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := s.Latest(ctx, "USD", "NGN")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if !got.Rate.Equal(decimal.NewFromFloat(1500)) {
		t.Errorf("rate = %s, want 1500", got.Rate)
	}
}
