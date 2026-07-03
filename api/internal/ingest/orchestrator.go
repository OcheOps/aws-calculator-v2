package ingest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/OcheOps/aws-calculator/api/internal/providers"
	"github.com/jackc/pgx/v5/pgxpool"
)

const advisoryLockKey int64 = 0x4157534341_4c43 // "AWSCALC"

type Orchestrator struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewOrchestrator(pool *pgxpool.Pool) *Orchestrator {
	return &Orchestrator{pool: pool, logger: slog.Default()}
}

func (o *Orchestrator) WithLogger(l *slog.Logger) *Orchestrator {
	o.logger = l
	return o
}

func (o *Orchestrator) Run(ctx context.Context, p providers.Provider) error {
	conn, err := o.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	var gotLock bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, advisoryLockKey).Scan(&gotLock); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	if !gotLock {
		return fmt.Errorf("another ingestion run is in progress")
	}
	defer conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, advisoryLockKey)

	providerID, err := providerIDByCode(ctx, o.pool, p.Metadata().Code)
	if err != nil {
		return err
	}
	var runID int64
	if err := o.pool.QueryRow(ctx, `INSERT INTO ingestion_runs (provider_id, status) VALUES ($1, 'running') RETURNING id`, providerID).Scan(&runID); err != nil {
		return fmt.Errorf("start run: %w", err)
	}
	o.logger.Info("ingestion started", "provider", p.Metadata().Code, "run_id", runID)

	// Seed staging from the current authoritative catalog. When the bulk
	// ingestor skips an unchanged file (ETag match) it writes nothing to
	// staging for that service+region — without seeding, the atomic swap
	// would delete those SKUs from sku_prices. Seeded rows are overwritten
	// naturally when the ingestor DOES process a file (per-service upsert
	// semantics via ON CONFLICT DO NOTHING against fresh rows).
	if _, err := o.pool.Exec(ctx, `TRUNCATE staging_sku_prices RESTART IDENTITY`); err != nil {
		return fmt.Errorf("truncate staging: %w", err)
	}
	if _, err := o.pool.Exec(ctx, `
		INSERT INTO staging_sku_prices (
			service_id, sku, region_code, unit, currency, price_per_unit,
			term_type, purchase_option, lease_length, offering_class,
			attributes, effective_from, effective_to
		)
		SELECT service_id, sku, region_code, unit, currency, price_per_unit,
		       term_type, purchase_option, lease_length, offering_class,
		       attributes, effective_from, effective_to
		FROM sku_prices`); err != nil {
		return fmt.Errorf("seed staging from sku_prices: %w", err)
	}

	err = p.Ingest(ctx, providers.IngestDeps{DB: o.pool, Logger: o.logger, RunID: runID})
	if err != nil {
		_, uerr := o.pool.Exec(ctx, `UPDATE ingestion_runs SET status='failed', finished_at=NOW(), error=$1 WHERE id=$2`, err.Error(), runID)
		if uerr != nil {
			o.logger.Error("failed to record failure", "err", uerr)
		}
		return fmt.Errorf("ingest: %w", err)
	}

	if err := SwapStaging(ctx, o.pool); err != nil {
		_, _ = o.pool.Exec(ctx, `UPDATE ingestion_runs SET status='failed', finished_at=NOW(), error=$1 WHERE id=$2`, err.Error(), runID)
		return fmt.Errorf("swap staging: %w", err)
	}

	if _, err := o.pool.Exec(ctx, `
		UPDATE ingestion_runs SET
			status='success',
			finished_at=NOW(),
			services_count=(SELECT COUNT(*) FROM services WHERE provider_id=$1),
			skus_count=(SELECT COUNT(*) FROM sku_prices sp JOIN services s ON s.id=sp.service_id WHERE s.provider_id=$1)
		WHERE id=$2`, providerID, runID); err != nil {
		return fmt.Errorf("finalize run: %w", err)
	}
	o.logger.Info("ingestion finished", "provider", p.Metadata().Code, "run_id", runID)
	return nil
}

func providerIDByCode(ctx context.Context, pool *pgxpool.Pool, code string) (int64, error) {
	var id int64
	if err := pool.QueryRow(ctx, `SELECT id FROM providers WHERE code=$1`, code).Scan(&id); err != nil {
		return 0, fmt.Errorf("provider %s not found: %w", code, err)
	}
	return id, nil
}
