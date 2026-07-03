package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/auth"
	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/go-chi/chi/v5"
)

func setupEstimateHandlers(t *testing.T) (*httptest.Server, int64) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, _ := db.Open(ctx, url)
	t.Cleanup(pool.Close)
	pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	db.Migrate(url)

	var providerID, serviceID int64
	pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1, 'AmazonEC2', 'EC2') RETURNING id`, providerID).Scan(&serviceID)

	estStore := estimates.NewStore(pool)
	authStore := auth.NewStore(pool)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		MountEstimates(r, EstimatesDeps{Store: estStore, AuthStore: authStore, BaseURL: "https://calc.example"})
		MountShared(r, estStore)
	})
	return httptest.NewServer(r), serviceID
}

func TestEstimates_AnonymousFlow(t *testing.T) {
	srv, svcID := setupEstimateHandlers(t)
	defer srv.Close()
	c := &http.Client{}

	body, _ := json.Marshal(map[string]string{"name": "anon test", "currency": "USD"})
	resp, err := c.Post(srv.URL+"/api/v1/estimates", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Data struct {
			Estimate  estimates.Estimate `json:"estimate"`
			EditToken string             `json:"edit_token"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || created.Data.EditToken == "" {
		t.Fatalf("create status=%d edit=%q", resp.StatusCode, created.Data.EditToken)
	}
	id := created.Data.Estimate.ID

	// Add an item
	itemBody, _ := json.Marshal(map[string]any{
		"service_id": svcID, "region_code": "us-east-1", "sku": "S",
		"quantity": "1", "attributes": map[string]string{"instanceType": "m5.large"},
		"unit_price": "0.096", "unit": "Hrs", "currency": "USD",
	})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/estimates/"+itoa(id)+"/items", bytes.NewReader(itemBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Edit-Token", created.Data.EditToken)
	resp, _ = c.Do(req)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add item status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Read back with edit token
	req, _ = http.NewRequest("GET", srv.URL+"/api/v1/estimates/"+itoa(id), nil)
	req.Header.Set("X-Edit-Token", created.Data.EditToken)
	resp, _ = c.Do(req)
	if resp.StatusCode != 200 {
		t.Fatalf("get status = %d", resp.StatusCode)
	}
	var got struct{ Data estimates.Estimate }
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if len(got.Data.Items) != 1 {
		t.Errorf("items = %d", len(got.Data.Items))
	}

	// Get without edit token → forbidden
	resp, _ = c.Get(srv.URL + "/api/v1/estimates/" + itoa(id))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("no-token status = %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()

	// Share
	req, _ = http.NewRequest("POST", srv.URL+"/api/v1/estimates/"+itoa(id)+"/share", nil)
	req.Header.Set("X-Edit-Token", created.Data.EditToken)
	resp, _ = c.Do(req)
	var shared struct {
		Data struct {
			ShareToken string `json:"share_token"`
			ShareURL   string `json:"share_url"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&shared)
	resp.Body.Close()
	if shared.Data.ShareToken == "" {
		t.Fatal("no share token")
	}

	// Read via /shared/{token}
	resp, _ = c.Get(srv.URL + "/api/v1/shared/" + shared.Data.ShareToken)
	if resp.StatusCode != 200 {
		t.Fatalf("shared status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Export as PDF
	req, _ = http.NewRequest("GET", srv.URL+"/api/v1/estimates/"+itoa(id)+"/export?format=pdf", nil)
	req.Header.Set("X-Edit-Token", created.Data.EditToken)
	resp, _ = c.Do(req)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/pdf" {
		t.Errorf("pdf status=%d ct=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp.Body.Close()
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
