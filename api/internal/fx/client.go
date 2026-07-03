package fx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{baseURL: baseURL, http: &http.Client{Timeout: 10 * time.Second}}
}

type fxResponse struct {
	Base  string             `json:"base"`
	Rates map[string]float64 `json:"rates"`
}

func (c *HTTPClient) Fetch(ctx context.Context, base string, quotes []string) ([]Rate, error) {
	url := fmt.Sprintf("%s/latest?base=%s&symbols=%s", c.baseURL, base, strings.Join(quotes, ","))
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
	now := time.Now().UTC()
	rates := make([]Rate, 0, len(body.Rates))
	for quote, v := range body.Rates {
		rates = append(rates, Rate{Base: base, Quote: quote, Rate: decimal.NewFromFloat(v), FetchedAt: now})
	}
	return rates, nil
}
