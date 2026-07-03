package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/go-chi/chi/v5"
)

func setupCatalogHandlers(t *testing.T) http.Handler {
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
	pool.Exec(ctx, `INSERT INTO service_dimensions (service_id, key, label, kind) VALUES ($1, 'instanceType', 'instanceType', 'enum')`, serviceID)
	pool.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type) VALUES ($1, 'K1', 'us-east-1', 'Hrs', 0.01, 'OnDemand')`, serviceID)

	r := chi.NewRouter()
	MountCatalog(r, catalog.NewStore(pool))
	return r
}

func TestCatalogHandlers_ListProviders(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/providers")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data []catalog.Provider `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 || body.Data[0].Code != "aws" {
		t.Errorf("got %+v", body.Data)
	}
}

func TestCatalogHandlers_ListServices(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/services?provider=aws")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Data []catalog.Service `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 || body.Data[0].Code != "AmazonEC2" {
		t.Errorf("got %+v", body.Data)
	}
}

func TestCatalogHandlers_GetService_IncludesDimensions(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/services/AmazonEC2?provider=aws")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data struct {
			Service    catalog.Service     `json:"service"`
			Dimensions []catalog.Dimension `json:"dimensions"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Data.Service.Code != "AmazonEC2" {
		t.Errorf("service = %+v", body.Data.Service)
	}
	if len(body.Data.Dimensions) != 1 || body.Data.Dimensions[0].Key != "instanceType" {
		t.Errorf("dimensions = %+v", body.Data.Dimensions)
	}
}

func TestCatalogHandlers_ListRegions(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/regions?provider=aws")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Data []catalog.Region `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 || body.Data[0].Code != "us-east-1" {
		t.Errorf("regions = %+v", body.Data)
	}
}

func TestCatalogHandlers_Currencies(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/currencies")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Data []string `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	wants := map[string]bool{"USD": true, "EUR": true, "GBP": true, "NGN": true}
	for _, c := range body.Data {
		delete(wants, c)
	}
	if len(wants) != 0 {
		t.Errorf("missing currencies: %v", wants)
	}
}
