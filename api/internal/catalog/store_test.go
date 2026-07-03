package catalog

import (
	"context"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
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
	var providerID, serviceID int64
	pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1, 'AmazonEC2', 'EC2') RETURNING id`, providerID).Scan(&serviceID)
	pool.Exec(ctx, `INSERT INTO service_dimensions (service_id, key, label, kind, sort_order) VALUES ($1, 'instanceType', 'instanceType', 'enum', 0)`, serviceID)
	pool.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type) VALUES ($1, 'K1', 'us-east-1', 'Hrs', 0.01, 'OnDemand'), ($1, 'K2', 'eu-west-1', 'Hrs', 0.02, 'OnDemand')`, serviceID)
	return NewStore(pool), ctx
}

func TestStore_ListProviders(t *testing.T) {
	s, ctx := setupStore(t)
	provs, err := s.ListProviders(ctx)
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(provs) != 1 || provs[0].Code != "aws" {
		t.Errorf("got %+v", provs)
	}
}

func TestStore_ListServices(t *testing.T) {
	s, ctx := setupStore(t)
	svcs, cursor, err := s.ListServices(ctx, ListServicesQuery{ProviderCode: "aws", Limit: 50})
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(svcs) != 1 || svcs[0].Code != "AmazonEC2" {
		t.Errorf("got %+v", svcs)
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}

func TestStore_ListRegions(t *testing.T) {
	s, ctx := setupStore(t)
	regions, err := s.ListRegions(ctx, "aws")
	if err != nil {
		t.Fatalf("ListRegions: %v", err)
	}
	if len(regions) != 2 {
		t.Errorf("regions = %v", regions)
	}
}

func TestStore_HasSuccessfulIngestion(t *testing.T) {
	s, ctx := setupStore(t)
	ok, err := s.HasSuccessfulIngestion(ctx)
	if err != nil {
		t.Fatalf("HasSuccessfulIngestion: %v", err)
	}
	if ok {
		t.Error("expected false before any run")
	}
	var providerID int64
	s.pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	s.pool.Exec(ctx, `INSERT INTO ingestion_runs (provider_id, status, finished_at) VALUES ($1, 'success', NOW())`, providerID)
	ok, err = s.HasSuccessfulIngestion(ctx)
	if err != nil {
		t.Fatalf("HasSuccessfulIngestion 2: %v", err)
	}
	if !ok {
		t.Error("expected true after success run")
	}
}
