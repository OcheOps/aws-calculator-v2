package pricing

import (
	"context"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
)

func setup(t *testing.T) (*Store, int64, context.Context) {
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
	pool.Exec(ctx, `INSERT INTO service_dimensions (service_id, key, label, kind, is_required, sort_order) VALUES ($1,'instanceType','','enum',true,0),($1,'operatingSystem','','enum',false,1),($1,'preInstalledSw','','enum',false,2)`, serviceID)
	pool.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type, attributes)
		VALUES
		($1,'SKU_EXACT','us-east-1','Hrs',0.096,'OnDemand','{"instanceType":"m5.large","operatingSystem":"Linux","preInstalledSw":"NA"}'::jsonb),
		($1,'SKU_PARTIAL','us-east-1','Hrs',0.100,'OnDemand','{"instanceType":"m5.large","operatingSystem":"Linux"}'::jsonb)`, serviceID)
	return NewStore(pool), serviceID, ctx
}

func TestStore_ExactMatch(t *testing.T) {
	s, id, ctx := setup(t)
	got, err := s.Find(ctx, LookupQuery{
		ServiceID: id, Region: "us-east-1", TermType: "OnDemand",
		Attributes: map[string]string{"instanceType": "m5.large", "operatingSystem": "Linux", "preInstalledSw": "NA"},
	})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.SKU != "SKU_EXACT" || got.MatchQuality != "exact" {
		t.Errorf("got %+v", got)
	}
}

func TestStore_PartialMatch(t *testing.T) {
	s, id, ctx := setup(t)
	got, err := s.Find(ctx, LookupQuery{
		ServiceID: id, Region: "us-east-1", TermType: "OnDemand",
		Attributes: map[string]string{"instanceType": "m5.large", "operatingSystem": "Linux", "preInstalledSw": "Xxx"},
	})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.MatchQuality != "partial" || len(got.DroppedAttrs) == 0 {
		t.Errorf("got %+v", got)
	}
}
