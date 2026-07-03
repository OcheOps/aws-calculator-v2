package fx

import (
	"context"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/shopspring/decimal"
)

type stubFetcher struct {
	rates []Rate
	err   error
}

func (s stubFetcher) Fetch(ctx context.Context, base string, quotes []string) ([]Rate, error) {
	return s.rates, s.err
}

func TestRefresher_WritesFetchedRates(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	defer pool.Close()
	_, _ = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	db.Migrate(url)

	fetcher := stubFetcher{rates: []Rate{{Base: "USD", Quote: "EUR", Rate: decimal.NewFromFloat(0.9)}}}
	r := NewRefresher(fetcher, NewStore(pool), []string{"EUR"})
	if err := r.Refresh(ctx); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	got, err := NewStore(pool).Latest(ctx, "USD", "EUR")
	if err != nil || !got.Rate.Equal(decimal.NewFromFloat(0.9)) {
		t.Errorf("got %+v err=%v", got, err)
	}
}
