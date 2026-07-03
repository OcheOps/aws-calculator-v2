package aws

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func loadFixture(t *testing.T, name string) []json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var strs []string
	if err := json.Unmarshal(data, &strs); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	out := make([]json.RawMessage, len(strs))
	for i, s := range strs {
		out[i] = json.RawMessage(s)
	}
	return out
}

type fixtureSDK struct{ products []json.RawMessage }

func (f *fixtureSDK) ListServiceCodes(ctx context.Context) ([]string, error) {
	return []string{"AmazonEC2"}, nil
}
func (f *fixtureSDK) DescribeService(ctx context.Context, code string) ([]string, error) {
	return []string{"instanceType", "regionCode", "operatingSystem", "tenancy", "preInstalledSw"}, nil
}
func (f *fixtureSDK) GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error) {
	return f.products, "", nil
}

func (f *fixtureSDK) ListPriceLists(ctx context.Context, serviceCode string, effective time.Time) ([]PriceListRef, error) {
	return nil, nil
}

func (f *fixtureSDK) GetPriceListFileURL(ctx context.Context, arn, format string) (string, error) {
	return "", nil
}

func TestIngest_PopulatesStagingAndDimensions(t *testing.T) {
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
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	client := NewClient(&fixtureSDK{products: loadFixture(t, "get_products_ec2_page1.json")})
	ing := NewIngestor(client, []string{"us-east-1"})

	if err := ing.Run(ctx, pool); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var services int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM services WHERE code='AmazonEC2'`).Scan(&services)
	if services != 1 {
		t.Errorf("services count = %d", services)
	}
	var stagingRows int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM staging_sku_prices`).Scan(&stagingRows)
	if stagingRows == 0 {
		t.Fatal("expected staging rows to be populated")
	}
	assertDim(t, pool, "instanceType", true)
	assertDim(t, pool, "regionCode", true)
}

func assertDim(t *testing.T, pool *pgxpool.Pool, key string, wantAny bool) {
	t.Helper()
	var count int
	pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM service_dimensions WHERE key=$1`, key).Scan(&count)
	if wantAny && count == 0 {
		t.Errorf("dimension %s missing", key)
	}
}
