package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

type Ingestor struct {
	client  *Client
	regions []string
	limiter *rate.Limiter
}

func NewIngestor(client *Client, regions []string) *Ingestor {
	return &Ingestor{
		client:  client,
		regions: regions,
		limiter: rate.NewLimiter(rate.Limit(8), 1),
	}
}

type productDoc struct {
	Product struct {
		SKU        string            `json:"sku"`
		Attributes map[string]string `json:"attributes"`
	} `json:"product"`
	Terms map[string]map[string]struct {
		PriceDimensions map[string]struct {
			Unit         string            `json:"unit"`
			PricePerUnit map[string]string `json:"pricePerUnit"`
		} `json:"priceDimensions"`
	} `json:"terms"`
}

func (i *Ingestor) Run(ctx context.Context, pool *pgxpool.Pool) error {
	if err := i.wait(ctx); err != nil {
		return err
	}
	services, err := i.client.ListServices(ctx)
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}
	providerID, err := lookupProviderID(ctx, pool)
	if err != nil {
		return err
	}
	slog.Info("ingest run", "services", len(services), "regions", i.regions)
	failed := 0
	for _, code := range services {
		if err := i.ingestService(ctx, pool, providerID, code); err != nil {
			// One broken service should not abort a multi-hour run; log and
			// continue so the staging swap still produces a mostly-complete
			// catalog.
			slog.Warn("ingest service failed; skipping", "service", code, "err", err)
			failed++
		}
	}
	if failed == len(services) && len(services) > 0 {
		return fmt.Errorf("all %d services failed to ingest", failed)
	}
	slog.Info("ingest run finished", "services", len(services), "failed", failed)
	return nil
}

func (i *Ingestor) ingestService(ctx context.Context, pool *pgxpool.Pool, providerID int64, code string) error {
	if err := i.wait(ctx); err != nil {
		return err
	}
	attrs, err := i.client.DescribeService(ctx, code)
	if err != nil {
		return err
	}
	serviceID, err := upsertService(ctx, pool, providerID, code)
	if err != nil {
		return err
	}
	if err := upsertDimensions(ctx, pool, serviceID, attrs); err != nil {
		return err
	}

	slog.Info("ingest service", "service", code, "regions", len(i.regions))
	for _, region := range i.regions {
		start := time.Now()
		if err := i.ingestRegion(ctx, pool, serviceID, code, region); err != nil {
			// Non-fatal: skip this service/region and continue the run so a
			// single stubborn combo can't fail the whole catalog refresh.
			slog.Warn("ingest region failed; skipping",
				"service", code, "region", region, "err", err, "elapsed", time.Since(start))
			continue
		}
		slog.Info("ingest region complete",
			"service", code, "region", region, "elapsed", time.Since(start))
	}
	return nil
}

func (i *Ingestor) ingestRegion(ctx context.Context, pool *pgxpool.Pool, serviceID int64, code, region string) error {
	// AWS Pricing NextTokens expire after ~5 minutes. Any retry that outlasts
	// that window will produce ExpiredNextTokenException. On that specific
	// error we restart the region from page 1; staging_sku_prices has
	// ON CONFLICT DO NOTHING so duplicate rows are absorbed.
	const maxRegionRestarts = 4
	for restart := 0; restart <= maxRegionRestarts; restart++ {
		token := ""
		for {
			if err := i.wait(ctx); err != nil {
				return err
			}
			products, next, err := i.getProductsWithRetry(ctx, code, region, token)
			if err != nil {
				if isExpiredToken(err) && restart < maxRegionRestarts {
					slog.Warn("aws pricing NextToken expired; restarting region",
						"service", code, "region", region, "restart", restart+1)
					break
				}
				return err
			}
			for _, raw := range products {
				if err := insertProduct(ctx, pool, serviceID, region, raw); err != nil {
					return err
				}
			}
			if next == "" {
				return nil
			}
			token = next
		}
	}
	return fmt.Errorf("service %s region %s: NextToken kept expiring after %d restarts", code, region, maxRegionRestarts)
}

func isExpiredToken(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ExpiredNextTokenException")
}

// getProductsWithRetry wraps GetProducts with a short exponential backoff for
// transient network errors — most commonly HTTP/2 GOAWAY frames the AWS
// Pricing endpoint sends on long-lived connections, which the SDK's default
// retryer does not classify as retryable.
func (i *Ingestor) getProductsWithRetry(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error) {
	const maxAttempts = 5
	var lastErr error
	backoff := 500 * time.Millisecond
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		products, next, err := i.client.GetProducts(ctx, code, region, token)
		if err == nil {
			return products, next, nil
		}
		lastErr = err
		if !isTransient(err) {
			return nil, "", err
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return nil, "", fmt.Errorf("get products %s/%s after %d attempts: %w", code, region, maxAttempts, lastErr)
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// HTTP/2 graceful shutdown, throttling, DNS blips, TCP resets.
	for _, needle := range []string{
		"GOAWAY",
		"stream closed",
		"connection reset",
		"EOF",
		"i/o timeout",
		"ThrottlingException",
		"TooManyRequestsException",
		"RequestTimeout",
		"deserialization failed",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func (i *Ingestor) wait(ctx context.Context) error {
	return i.limiter.Wait(ctx)
}

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

func insertProduct(ctx context.Context, pool *pgxpool.Pool, serviceID int64, region string, raw json.RawMessage) error {
	var doc productDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse product: %w", err)
	}
	if doc.Product.SKU == "" {
		return nil
	}
	attrJSON, err := json.Marshal(doc.Product.Attributes)
	if err != nil {
		return err
	}
	for termType, offers := range doc.Terms {
		for _, offer := range offers {
			for _, dim := range offer.PriceDimensions {
				price, ok := dim.PricePerUnit["USD"]
				if !ok {
					continue
				}
				_, err := pool.Exec(ctx, `
					INSERT INTO staging_sku_prices (service_id, sku, region_code, unit, currency, price_per_unit, term_type, attributes, effective_from)
					VALUES ($1, $2, $3, $4, 'USD', $5::numeric, $6, $7, $8)
					ON CONFLICT DO NOTHING`,
					serviceID, doc.Product.SKU, region, dim.Unit, price, termType, attrJSON, time.Now().UTC())
				if err != nil {
					return fmt.Errorf("insert sku %s: %w", doc.Product.SKU, err)
				}
			}
		}
	}
	for key, val := range doc.Product.Attributes {
		if val == "" {
			continue
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO dimension_values (dimension_id, value)
			SELECT id, $2 FROM service_dimensions WHERE service_id = $1 AND key = $3
			ON CONFLICT DO NOTHING`,
			serviceID, val, key)
		if err != nil {
			return fmt.Errorf("insert dim value %s=%s: %w", key, val, err)
		}
	}
	return nil
}
