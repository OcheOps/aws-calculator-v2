package pricing

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/OcheOps/aws-calculator/api/internal/fx"
	"github.com/shopspring/decimal"
)

func setupLookup(t *testing.T) (*Lookup, int64, context.Context) {
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
	db.Migrate(url)
	var providerID, serviceID int64
	pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1,'AmazonEC2','EC2') RETURNING id`, providerID).Scan(&serviceID)
	pool.Exec(ctx, `INSERT INTO service_dimensions (service_id, key, label, kind, is_required) VALUES ($1,'instanceType','','enum',true)`, serviceID)
	pool.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type, attributes)
		VALUES ($1,'SK','us-east-1','Hrs',1.00,'OnDemand','{"instanceType":"m5.large"}'::jsonb)`, serviceID)
	fxStore := fx.NewStore(pool)
	fxStore.Upsert(ctx, []fx.Rate{{Base: "USD", Quote: "NGN", Rate: decimal.NewFromFloat(1500)}})
	cache, _ := NewCache(1<<10, 5*time.Minute)
	catalogStore := catalog.NewStore(pool)
	return NewLookup(NewStore(pool), fxStore, cache, catalogStore), serviceID, ctx
}

func TestLookup_ConvertsCurrency(t *testing.T) {
	l, _, ctx := setupLookup(t)
	got, err := l.Price(ctx, LookupInput{
		ServiceCode: "AmazonEC2",
		Provider:    "aws",
		Region:      "us-east-1",
		TermType:    "OnDemand",
		Attributes:  map[string]string{"instanceType": "m5.large"},
		Currency:    "NGN",
	})
	if err != nil {
		t.Fatalf("Price: %v", err)
	}
	if !got.PricePerUnit.Equal(decimal.NewFromFloat(1500)) {
		t.Errorf("price = %s, want 1500", got.PricePerUnit)
	}
	if got.Currency != "NGN" {
		t.Errorf("currency = %s", got.Currency)
	}
}
