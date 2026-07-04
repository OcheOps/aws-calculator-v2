package estimates

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/OcheOps/aws-calculator/api/internal/fx"
	"github.com/OcheOps/aws-calculator/api/internal/pricing"
	"github.com/shopspring/decimal"
)

func TestRepricer_CuratedLineSumsComponents(t *testing.T) {
	// Fresh setup: two SKUs (compute + storage) so a curated 2-component
	// line has something to sum.
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

	var providerID, ec2ID int64
	pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1,'AmazonEC2','EC2') RETURNING id`, providerID).Scan(&ec2ID)
	pool.Exec(ctx, `INSERT INTO service_dimensions (service_id, key, label, kind) VALUES ($1,'instanceType','','enum')`, ec2ID)
	pool.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type, attributes)
		VALUES ($1,'EC2-COMPUTE','us-east-1','Hrs',0.096,'OnDemand','{"instanceType":"m5.large"}'::jsonb),
		       ($1,'EBS-STORE','us-east-1','GB-Mo',0.10,'OnDemand','{"volumeType":"gp3"}'::jsonb)`, ec2ID)

	cache, _ := pricing.NewCache(1<<10, time.Minute)
	lookup := pricing.NewLookup(pricing.NewStore(pool), fx.NewStore(pool), cache, catalog.NewStore(pool))
	store := NewStore(pool)

	var orgID int64
	pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('t','t') RETURNING id`).Scan(&orgID)
	est, _, _ := store.Create(ctx, CreateInput{Name: "x", Currency: "USD", OrgID: &orgID})
	_, err = store.AddCuratedItem(ctx, est.ID, AddCuratedItemInput{
		ServiceID:    ec2ID,
		ServiceKey:   "ec2",
		RegionCode:   "us-east-1",
		SyntheticSKU: "curated:ec2:v1",
		Currency:     "USD",
		UserInputSnapshot: map[string]any{"instanceType": "m5.large"},
		Components: []Component{
			{
				SKU: "EC2-COMPUTE", Unit: "Hrs",
				Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(0.096),
				MonthlyCost: decimal.NewFromFloat(70.08), Currency: "USD",
				Attributes: map[string]string{"instanceType": "m5.large"},
				Region:     "us-east-1", Service: "AmazonEC2",
			},
			{
				SKU: "EBS-STORE", Unit: "GB-Mo",
				Quantity: decimal.NewFromInt(100), UnitPrice: decimal.NewFromFloat(0.10),
				MonthlyCost: decimal.NewFromFloat(10.0), Currency: "USD",
				Attributes: map[string]string{"volumeType": "gp3"},
				Region:     "us-east-1", Service: "AmazonEC2",
			},
		},
	})
	if err != nil {
		t.Fatalf("AddCuratedItem: %v", err)
	}

	// Mutate the underlying compute price.
	pool.Exec(ctx, `UPDATE sku_prices SET price_per_unit = 0.192 WHERE sku='EC2-COMPUTE'`)

	rep := NewRepricer(store, lookup)
	deltas, err := rep.Preview(ctx, est.ID)
	if err != nil || len(deltas) != 1 {
		t.Fatalf("Preview: err=%v deltas=%d", err, len(deltas))
	}
	// New monthly = 0.192 * 730 + 0.10 * 100 = 140.16 + 10 = 150.16
	if !deltas[0].NewMonthly.Equal(decimal.NewFromFloat(150.16)) {
		t.Errorf("new monthly = %s, want 150.16", deltas[0].NewMonthly)
	}
	if !deltas[0].OldMonthly.Equal(decimal.NewFromFloat(80.08)) {
		t.Errorf("old monthly = %s, want 80.08", deltas[0].OldMonthly)
	}
}
