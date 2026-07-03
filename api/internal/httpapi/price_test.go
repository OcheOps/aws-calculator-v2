package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/OcheOps/aws-calculator/api/internal/fx"
	"github.com/OcheOps/aws-calculator/api/internal/pricing"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

func TestPriceHandler_ReturnsUSD(t *testing.T) {
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
	var pid, sid int64
	pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&pid)
	pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1,'AmazonEC2','EC2') RETURNING id`, pid).Scan(&sid)
	pool.Exec(ctx, `INSERT INTO service_dimensions (service_id, key, label, kind, is_required) VALUES ($1,'instanceType','','enum',true)`, sid)
	pool.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type, attributes) VALUES ($1,'S','us-east-1','Hrs',0.096,'OnDemand','{"instanceType":"m5.large"}'::jsonb)`, sid)
	fx.NewStore(pool).Upsert(ctx, []fx.Rate{{Base: "USD", Quote: "NGN", Rate: decimal.NewFromFloat(1500)}})

	cache, _ := pricing.NewCache(1<<10, time.Minute)
	l := pricing.NewLookup(pricing.NewStore(pool), fx.NewStore(pool), cache, catalog.NewStore(pool))

	r := chi.NewRouter()
	MountPricing(r, l)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/price?service=AmazonEC2&provider=aws&region=us-east-1&term=OnDemand&currency=USD&attr[instanceType]=m5.large")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data pricing.PriceResponse `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Data.Currency != "USD" || body.Data.SKU != "S" {
		t.Errorf("got %+v", body.Data)
	}
}
