package fx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_FetchesUSDRates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("base"); got != "USD" {
			t.Errorf("base = %q, want USD", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"base":"USD","rates":{"EUR":0.92,"NGN":1500,"GBP":0.78}}`))
	}))
	defer server.Close()

	c := NewHTTPClient(server.URL)
	rates, err := c.Fetch(context.Background(), "USD", []string{"EUR", "NGN", "GBP"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(rates) != 3 {
		t.Fatalf("rates = %d, want 3", len(rates))
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
