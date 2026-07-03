package aws

import (
	"context"
	"encoding/json"
	"fmt"
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
	for _, code := range services {
		if err := i.ingestService(ctx, pool, providerID, code); err != nil {
			return fmt.Errorf("service %s: %w", code, err)
		}
	}
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

	for _, region := range i.regions {
		if err := i.ingestRegion(ctx, pool, serviceID, code, region); err != nil {
			return fmt.Errorf("region %s: %w", region, err)
		}
	}
	return nil
}

func (i *Ingestor) ingestRegion(ctx context.Context, pool *pgxpool.Pool, serviceID int64, code, region string) error {
	token := ""
	for {
		if err := i.wait(ctx); err != nil {
			return err
		}
		products, next, err := i.client.GetProducts(ctx, code, region, token)
		if err != nil {
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
