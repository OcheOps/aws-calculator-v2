package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type dim struct {
	Key        string
	IsRequired bool
	SortOrder  int
}

func (s *Store) Find(ctx context.Context, q LookupQuery) (*LookupResult, error) {
	dims, err := s.dimensions(ctx, q.ServiceID)
	if err != nil {
		return nil, err
	}
	if err := s.checkRequired(q.Attributes, dims); err != nil {
		return nil, err
	}

	if r, err := s.queryOne(ctx, q, q.Attributes); err == nil {
		r.MatchQuality = "exact"
		return r, nil
	}

	optional := make([]string, 0)
	for _, d := range dims {
		if !d.IsRequired {
			if _, present := q.Attributes[d.Key]; present {
				optional = append(optional, d.Key)
			}
		}
	}
	sort.SliceStable(optional, func(i, j int) bool {
		return dimSortOrder(dims, optional[i]) > dimSortOrder(dims, optional[j])
	})

	dropped := make([]string, 0, len(optional))
	attrs := copyMap(q.Attributes)
	for _, k := range optional {
		delete(attrs, k)
		dropped = append(dropped, k)
		if r, err := s.queryOne(ctx, q, attrs); err == nil {
			r.MatchQuality = "partial"
			r.DroppedAttrs = dropped
			return r, nil
		}
	}
	return nil, fmt.Errorf("no matching sku")
}

func (s *Store) queryOne(ctx context.Context, q LookupQuery, attrs map[string]string) (*LookupResult, error) {
	attrJSON, err := json.Marshal(attrs)
	if err != nil {
		return nil, err
	}
	var (
		sku, unit, currency, term string
		priceStr                  string
	)
	err = s.pool.QueryRow(ctx, `
		SELECT sku, unit, currency, price_per_unit::text, term_type
		FROM sku_prices
		WHERE service_id=$1 AND region_code=$2 AND term_type=$3 AND attributes @> $4::jsonb
		ORDER BY price_per_unit ASC
		LIMIT 1`, q.ServiceID, q.Region, q.TermType, attrJSON).Scan(&sku, &unit, &currency, &priceStr, &term)
	if err != nil {
		return nil, err
	}
	price, err := decimal.NewFromString(priceStr)
	if err != nil {
		return nil, err
	}
	return &LookupResult{SKU: sku, Unit: unit, Currency: currency, PricePerUnit: price, TermType: term}, nil
}

func (s *Store) dimensions(ctx context.Context, serviceID int64) ([]dim, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, is_required, sort_order FROM service_dimensions WHERE service_id=$1`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dim
	for rows.Next() {
		var d dim
		if err := rows.Scan(&d.Key, &d.IsRequired, &d.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) checkRequired(attrs map[string]string, dims []dim) error {
	for _, d := range dims {
		if d.IsRequired {
			if _, ok := attrs[d.Key]; !ok {
				return fmt.Errorf("required attribute %q missing", d.Key)
			}
		}
	}
	return nil
}

func dimSortOrder(dims []dim, key string) int {
	for _, d := range dims {
		if d.Key == key {
			return d.SortOrder
		}
	}
	return 0
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
