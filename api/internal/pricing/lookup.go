package pricing

import (
	"context"
	"fmt"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/fx"
	"github.com/shopspring/decimal"
)

type LookupInput struct {
	Provider    string
	ServiceCode string
	Region      string
	TermType    string
	Attributes  map[string]string
	Currency    string
}

type PriceResponse struct {
	*LookupResult
	Currency string          `json:"currency"`
	FXRate   decimal.Decimal `json:"fx_rate,omitempty"`
	FXAsOf   *time.Time      `json:"fx_as_of,omitempty"`
}

type Lookup struct {
	store   *Store
	fx      *fx.Store
	cache   *Cache
	catalog *catalog.Store
}

func NewLookup(s *Store, f *fx.Store, c *Cache, cat *catalog.Store) *Lookup {
	return &Lookup{store: s, fx: f, cache: c, catalog: cat}
}

func (l *Lookup) Price(ctx context.Context, in LookupInput) (*PriceResponse, error) {
	if in.TermType == "" {
		in.TermType = "OnDemand"
	}
	if in.Currency == "" {
		in.Currency = "USD"
	}
	if in.Provider == "" {
		in.Provider = "aws"
	}
	svc, err := l.catalog.GetServiceByCode(ctx, in.Provider, in.ServiceCode)
	if err != nil {
		return nil, fmt.Errorf("service not found: %w", err)
	}

	key := CacheKey(in.ServiceCode, in.Region, in.TermType, in.Attributes) + "|" + in.Currency
	if cached, ok := l.cache.Get(key); ok {
		return l.wrap(ctx, cached, in.Currency)
	}

	res, err := l.store.Find(ctx, LookupQuery{
		ServiceID: svc.ID, ServiceCode: in.ServiceCode, Region: in.Region,
		TermType: in.TermType, Attributes: in.Attributes,
	})
	if err != nil {
		return nil, err
	}
	l.cache.Set(key, res)
	return l.wrap(ctx, res, in.Currency)
}

func (l *Lookup) wrap(ctx context.Context, res *LookupResult, currency string) (*PriceResponse, error) {
	resp := &PriceResponse{LookupResult: res, Currency: currency}
	if currency == res.Currency {
		return resp, nil
	}
	rate, err := l.fx.Latest(ctx, res.Currency, currency)
	if err != nil {
		return nil, fmt.Errorf("no fx rate for %s/%s", res.Currency, currency)
	}
	converted := res.PricePerUnit.Mul(rate.Rate)
	newRes := *res
	newRes.PricePerUnit = converted
	newRes.Currency = currency
	resp.LookupResult = &newRes
	resp.FXRate = rate.Rate
	fetched := rate.FetchedAt
	resp.FXAsOf = &fetched
	return resp, nil
}
