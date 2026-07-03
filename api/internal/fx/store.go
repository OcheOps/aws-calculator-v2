package fx

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Upsert(ctx context.Context, rates []Rate) error {
	for _, r := range rates {
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO fx_rates (base, quote, rate) VALUES ($1, $2, $3::numeric)`,
			r.Base, r.Quote, r.Rate.String()); err != nil {
			return fmt.Errorf("insert fx %s/%s: %w", r.Base, r.Quote, err)
		}
	}
	return nil
}

func (s *Store) Latest(ctx context.Context, base, quote string) (Rate, error) {
	var rateStr string
	var r Rate
	err := s.pool.QueryRow(ctx,
		`SELECT rate::text, fetched_at FROM fx_rates WHERE base=$1 AND quote=$2 ORDER BY fetched_at DESC LIMIT 1`,
		base, quote).Scan(&rateStr, &r.FetchedAt)
	if err != nil {
		return r, fmt.Errorf("no rate for %s/%s: %w", base, quote, err)
	}
	dec, err := decimal.NewFromString(rateStr)
	if err != nil {
		return r, fmt.Errorf("parse rate: %w", err)
	}
	r.Base, r.Quote, r.Rate = base, quote, dec
	return r, nil
}
