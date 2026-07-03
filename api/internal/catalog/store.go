package catalog

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := s.pool.Query(ctx, `SELECT code, name FROM providers WHERE enabled ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Provider
	for rows.Next() {
		var p Provider
		if err := rows.Scan(&p.Code, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type ListServicesQuery struct {
	ProviderCode string
	Search       string
	Cursor       string
	Limit        int
}

func (s *Store) ListServices(ctx context.Context, q ListServicesQuery) ([]Service, string, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	afterID, err := decodeCursor(q.Cursor)
	if err != nil {
		return nil, "", err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.provider_id, s.code, s.name, COALESCE(s.family,''), COALESCE(s.description,'')
		FROM services s
		JOIN providers p ON p.id = s.provider_id
		WHERE p.code = $1
		  AND ($2::text = '' OR s.code ILIKE '%' || $2 || '%' OR s.name ILIKE '%' || $2 || '%')
		  AND s.id > $3
		ORDER BY s.id
		LIMIT $4`, q.ProviderCode, q.Search, afterID, q.Limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	svcs := make([]Service, 0, q.Limit)
	for rows.Next() {
		var svc Service
		if err := rows.Scan(&svc.ID, &svc.ProviderID, &svc.Code, &svc.Name, &svc.Family, &svc.Description); err != nil {
			return nil, "", err
		}
		svcs = append(svcs, svc)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(svcs) > q.Limit {
		nextCursor = encodeCursor(svcs[q.Limit-1].ID)
		svcs = svcs[:q.Limit]
	}
	return svcs, nextCursor, nil
}

func (s *Store) GetServiceByCode(ctx context.Context, providerCode, serviceCode string) (*Service, error) {
	var svc Service
	err := s.pool.QueryRow(ctx, `
		SELECT s.id, s.provider_id, s.code, s.name, COALESCE(s.family,''), COALESCE(s.description,'')
		FROM services s JOIN providers p ON p.id = s.provider_id
		WHERE p.code=$1 AND s.code=$2`, providerCode, serviceCode).
		Scan(&svc.ID, &svc.ProviderID, &svc.Code, &svc.Name, &svc.Family, &svc.Description)
	if err != nil {
		return nil, err
	}
	return &svc, nil
}

func (s *Store) ListDimensions(ctx context.Context, serviceID int64) ([]Dimension, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, key, label, kind, value_type, is_required, sort_order
		FROM service_dimensions
		WHERE service_id=$1
		ORDER BY sort_order, key`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Dimension
	for rows.Next() {
		var d Dimension
		if err := rows.Scan(&d.ID, &d.Key, &d.Label, &d.Kind, &d.ValueType, &d.IsRequired, &d.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) ListDimensionValues(ctx context.Context, dimensionID int64) ([]DimensionValue, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT value, COALESCE(label,'') FROM dimension_values WHERE dimension_id=$1 ORDER BY value`, dimensionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DimensionValue
	for rows.Next() {
		var v DimensionValue
		if err := rows.Scan(&v.Value, &v.Label); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) ListRegions(ctx context.Context, providerCode string) ([]Region, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT region_code FROM sku_prices sp
		JOIN services s ON s.id = sp.service_id
		JOIN providers p ON p.id = s.provider_id
		WHERE p.code = $1
		ORDER BY region_code`, providerCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Region
	for rows.Next() {
		var r Region
		if err := rows.Scan(&r.Code); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) HasSuccessfulIngestion(ctx context.Context) (bool, error) {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ingestion_runs WHERE status='success'`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func encodeCursor(id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
}

func decodeCursor(cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor: %w", err)
	}
	return strconv.ParseInt(string(b), 10, 64)
}
