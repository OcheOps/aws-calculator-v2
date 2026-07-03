package ingest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SwapStaging atomically replaces sku_prices with the contents of
// staging_sku_prices, then empties staging. Callers should have finished
// writing all staging rows before calling this.
func SwapStaging(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `TRUNCATE sku_prices RESTART IDENTITY`); err != nil {
			return fmt.Errorf("truncate sku_prices: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, currency, price_per_unit, term_type, purchase_option, lease_length, offering_class, attributes, effective_from, effective_to)
			SELECT service_id, sku, region_code, unit, currency, price_per_unit, term_type, purchase_option, lease_length, offering_class, attributes, effective_from, effective_to FROM staging_sku_prices`); err != nil {
			return fmt.Errorf("copy staging to sku_prices: %w", err)
		}
		if _, err := tx.Exec(ctx, `TRUNCATE staging_sku_prices RESTART IDENTITY`); err != nil {
			return fmt.Errorf("truncate staging: %w", err)
		}
		return nil
	})
}
