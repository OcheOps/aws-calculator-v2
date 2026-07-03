package ingest

import (
	"context"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbHandle struct {
	pool      any
	serviceID int64
}

func setupCatalogDB(t *testing.T) *dbHandle {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	// Ensure a clean slate: drop and re-migrate.
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS staging_sku_prices, sku_prices, dimension_values, service_dimensions, services, ingestion_runs, fx_rates, providers, schema_migrations CASCADE`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Seed a provider and service so FK checks pass.
	var providerID, serviceID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1, 'AmazonEC2', 'EC2') RETURNING id`, providerID).Scan(&serviceID); err != nil {
		t.Fatalf("seed service: %v", err)
	}
	return &dbHandle{pool: pool, serviceID: serviceID}
}

func getPool(t *testing.T, h *dbHandle) *pgxpool.Pool {
	t.Helper()
	p, ok := h.pool.(*pgxpool.Pool)
	if !ok {
		t.Fatalf("unexpected pool type: %T", h.pool)
	}
	return p
}

func TestSwapStaging_ReplacesRows(t *testing.T) {
	h := setupCatalogDB(t)
	ctx := context.Background()
	pool2 := getPool(t, h)

	if _, err := pool2.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type) VALUES ($1, 'OLD1', 'us-east-1', 'Hrs', 0.01, 'OnDemand'), ($1, 'OLD2', 'us-east-1', 'Hrs', 0.02, 'OnDemand')`, h.serviceID); err != nil {
		t.Fatalf("seed sku_prices: %v", err)
	}
	if _, err := pool2.Exec(ctx, `INSERT INTO staging_sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type) VALUES ($1, 'NEW1', 'us-east-1', 'Hrs', 0.03, 'OnDemand')`, h.serviceID); err != nil {
		t.Fatalf("seed staging: %v", err)
	}

	if err := SwapStaging(ctx, pool2); err != nil {
		t.Fatalf("SwapStaging: %v", err)
	}

	var count int
	if err := pool2.QueryRow(ctx, `SELECT COUNT(*) FROM sku_prices`).Scan(&count); err != nil {
		t.Fatalf("count sku_prices: %v", err)
	}
	if count != 1 {
		t.Errorf("sku_prices count = %d, want 1", count)
	}
	var sku string
	if err := pool2.QueryRow(ctx, `SELECT sku FROM sku_prices`).Scan(&sku); err != nil {
		t.Fatalf("read sku: %v", err)
	}
	if sku != "NEW1" {
		t.Errorf("sku = %q, want NEW1", sku)
	}
	var stagingCount int
	if err := pool2.QueryRow(ctx, `SELECT COUNT(*) FROM staging_sku_prices`).Scan(&stagingCount); err != nil {
		t.Fatalf("count staging: %v", err)
	}
	if stagingCount != 0 {
		t.Errorf("staging count = %d, want 0", stagingCount)
	}
}
