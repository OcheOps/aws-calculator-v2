package fx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_FetchesUSDRates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v6/latest/USD") {
			t.Errorf("path = %q, want /v6/latest/USD suffix", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"success","base_code":"USD","rates":{"EUR":0.92,"NGN":1500,"GBP":0.78,"JPY":150}}`))
	}))
	defer server.Close()

	c := NewHTTPClient(server.URL)
	rates, err := c.Fetch(context.Background(), "USD", []string{"EUR", "NGN", "GBP"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(rates) != 3 {
		t.Fatalf("rates = %d, want 3 (JPY should be filtered out)", len(rates))
	}
}

func TestClient_FetchesAllWhenQuotesEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"success","base_code":"USD","rates":{"EUR":0.92,"NGN":1500}}`))
	}))
	defer server.Close()

	c := NewHTTPClient(server.URL)
	rates, err := c.Fetch(context.Background(), "USD", nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(rates) != 2 {
		t.Errorf("rates = %d, want 2", len(rates))
	}
}

func TestClient_HandlesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	c := NewHTTPClient(server.URL)
	if _, err := c.Fetch(context.Background(), "USD", []string{"EUR"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_HandlesResultError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"error","error-type":"invalid-key"}`))
	}))
	defer server.Close()
	c := NewHTTPClient(server.URL)
	if _, err := c.Fetch(context.Background(), "USD", nil); err == nil {
		t.Fatal("expected error when result != success")
	}
}
