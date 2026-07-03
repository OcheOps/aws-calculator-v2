package pricefiles

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Record struct {
	ProviderCode string
	ServiceCode  string
	RegionCode   string
	VersionID    string
	ETag         string
	URL          string
	FetchedAt    time.Time
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(p *pgxpool.Pool) *Store { return &Store{pool: p} }

// Get returns the last-known ETag for a service+region under a provider code,
// or ("", false, nil) when no record exists yet.
func (s *Store) Get(ctx context.Context, providerCode, serviceCode, regionCode string) (string, bool, error) {
	var etag string
	err := s.pool.QueryRow(ctx, `
		SELECT ipf.etag FROM ingestion_price_files ipf
		JOIN providers p ON p.id = ipf.provider_id
		WHERE p.code = $1 AND ipf.service_code = $2 AND ipf.region_code = $3`,
		providerCode, serviceCode, regionCode).Scan(&etag)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return etag, true, nil
}

func (s *Store) Upsert(ctx context.Context, r Record) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ingestion_price_files (provider_id, service_code, region_code, version_id, etag, url, fetched_at)
		SELECT p.id, $2, $3, $4, $5, $6, NOW() FROM providers p WHERE p.code = $1
		ON CONFLICT (provider_id, service_code, region_code)
		DO UPDATE SET version_id = EXCLUDED.version_id, etag = EXCLUDED.etag, url = EXCLUDED.url, fetched_at = NOW()`,
		r.ProviderCode, r.ServiceCode, r.RegionCode, r.VersionID, r.ETag, r.URL)
	return err
}

func (s *Store) Delete(ctx context.Context, providerCode, serviceCode, regionCode string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM ingestion_price_files ipf USING providers p
		WHERE ipf.provider_id = p.id AND p.code = $1 AND ipf.service_code = $2 AND ipf.region_code = $3`,
		providerCode, serviceCode, regionCode)
	return err
}
