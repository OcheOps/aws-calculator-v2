package fx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

type Fetcher interface {
	Fetch(ctx context.Context, base string, quotes []string) ([]Rate, error)
}

type HTTPClient struct {
	baseURL string
	http    *http.Client
}

// NewHTTPClient targets the open.er-api.com REST shape:
//
//	GET {baseURL}/v6/latest/{base}
//	→ {"result":"success","base_code":"USD","rates":{"EUR":0.92,"NGN":1500,…}}
//
// Callers can pass an httptest.Server base URL for tests. The `quotes`
// argument to Fetch filters the returned map — the API itself returns all
// currencies.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{baseURL: baseURL, http: &http.Client{Timeout: 10 * time.Second}}
}

type fxResponse struct {
	Result   string             `json:"result"`
	BaseCode string             `json:"base_code"`
	Base     string             `json:"base"` // legacy exchangerate.host shape (tests still use this)
	Rates    map[string]float64 `json:"rates"`
}

func (c *HTTPClient) Fetch(ctx context.Context, base string, quotes []string) ([]Rate, error) {
	url := fmt.Sprintf("%s/v6/latest/%s", c.baseURL, base)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fx fetch %d", resp.StatusCode)
	}
	var body fxResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Result != "" && body.Result != "success" {
		return nil, fmt.Errorf("fx api returned result=%q", body.Result)
	}
	now := time.Now().UTC()
	// If a filter is provided, keep only requested quotes; otherwise return all.
	want := map[string]bool{}
	for _, q := range quotes {
		want[q] = true
	}
	rates := make([]Rate, 0, len(body.Rates))
	for quote, v := range body.Rates {
		if len(want) > 0 && !want[quote] {
			continue
		}
		rates = append(rates, Rate{Base: base, Quote: quote, Rate: decimal.NewFromFloat(v), FetchedAt: now})
	}
	return rates, nil
}
