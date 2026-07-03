package aws

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This file holds the small helpers that the AWS ingestor (see bulk_ingest.go)
// and its tests share. The legacy paged Ingestor was removed in Plan 08 once
// BulkIngestor proved reliable.

func lookupProviderID(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var id int64
	err := pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("lookup provider: %w", err)
	}
	return id, nil
}

func upsertService(ctx context.Context, pool *pgxpool.Pool, providerID int64, code string) (int64, error) {
	var id int64
	err := pool.QueryRow(ctx, `
		INSERT INTO services (provider_id, code, name)
		VALUES ($1, $2, $2)
		ON CONFLICT (provider_id, code) DO UPDATE SET updated_at = NOW()
		RETURNING id`, providerID, code).Scan(&id)
	return id, err
}

func upsertDimensions(ctx context.Context, pool *pgxpool.Pool, serviceID int64, attrs []string) error {
	for i, key := range attrs {
		kind := "enum"
		if key == "regionCode" || key == "location" {
			kind = "region"
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO service_dimensions (service_id, key, label, kind, sort_order)
			VALUES ($1, $2, $2, $3, $4)
			ON CONFLICT (service_id, key) DO UPDATE SET sort_order = EXCLUDED.sort_order`,
			serviceID, key, kind, i)
		if err != nil {
			return fmt.Errorf("upsert dimension %s: %w", key, err)
		}
	}
	return nil
}
