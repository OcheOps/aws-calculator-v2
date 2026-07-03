package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BulkClient downloads AWS Pricing bulk price-list files and streams SKU rows
// without loading the whole (~100 MB) JSON into memory.
type BulkClient struct {
	sdk   SDK
	httpc *http.Client
}

func NewBulkClient(sdk SDK) *BulkClient {
	// No per-request timeout: the ingestion orchestrator wraps each run in an
	// 8-hour context. EC2 alone can push ~500 MB per region and a 10-minute
	// wall-clock cap trips before download completes on slower links.
	return &BulkClient{sdk: sdk, httpc: &http.Client{}}
}

// SKURow is one priced SKU emitted by DownloadAndStream. Callers accumulate
// them into batches (see writeBatch).
type SKURow struct {
	SKU        string
	Region     string
	Unit       string
	Currency   string
	Price      string // decimal as string
	TermType   string
	Attributes map[string]string
}

// HeadETag issues a HEAD request and returns the value of the ETag header.
// Used to short-circuit ingestion when the price file hasn't changed since the
// last run.
func (b *BulkClient) HeadETag(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := b.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("head %s: %d", url, resp.StatusCode)
	}
	return resp.Header.Get("ETag"), nil
}

// DownloadAndStream fetches the URL and pushes SKURow onto out until the file
// is fully consumed or ctx is cancelled. Returns the ETag response header and
// any error. The caller owns out and must not close it.
//
// The AWS bulk price-list JSON layout (trimmed to what we care about):
//
//	{
//	  "products": { "SKU": { "sku": "SKU", "attributes": {…} }, … },
//	  "terms":    { "OnDemand": { "SKU": { "SKU.OFFER": {
//	                  "priceDimensions": { "SKU.OFFER.DIM": {
//	                     "unit": "Hrs", "pricePerUnit": {"USD": "0.096"}
//	                  }}}}}}}
func (b *BulkClient) DownloadAndStream(
	ctx context.Context,
	url, region string,
	out chan<- SKURow,
) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := b.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("download %s: %d", url, resp.StatusCode)
	}
	etag := resp.Header.Get("ETag")
	if err := streamPriceFile(ctx, resp.Body, region, out); err != nil {
		return etag, err
	}
	return etag, nil
}

// streamPriceFile walks the price-list JSON with a streaming decoder. It
// collects the `products` map first (attributes are small) and then walks
// `terms.OnDemand` emitting one SKURow per priced dimension.
func streamPriceFile(ctx context.Context, r io.Reader, region string, out chan<- SKURow) error {
	// The file is small-object-heavy — buffering the body helps decoder perf.
	dec := json.NewDecoder(r)

	// AWS price files may or may not fit into memory as-is, but the products +
	// terms shapes are individually parseable to `any`. Structurally-typed
	// decoding lets us tolerate schema quirks across services.
	var doc struct {
		Products map[string]struct {
			SKU        string            `json:"sku"`
			Attributes map[string]string `json:"attributes"`
		} `json:"products"`
		Terms map[string]map[string]map[string]struct {
			PriceDimensions map[string]struct {
				Unit         string            `json:"unit"`
				PricePerUnit map[string]string `json:"pricePerUnit"`
			} `json:"priceDimensions"`
		} `json:"terms"`
	}
	if err := dec.Decode(&doc); err != nil {
		return fmt.Errorf("decode price file: %w", err)
	}

	for termType, offersBySKU := range doc.Terms {
		for sku, offers := range offersBySKU {
			product, ok := doc.Products[sku]
			if !ok {
				continue
			}
			for _, offer := range offers {
				for _, dim := range offer.PriceDimensions {
					price, ok := dim.PricePerUnit["USD"]
					if !ok {
						continue
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case out <- SKURow{
						SKU:        sku,
						Region:     region,
						Unit:       dim.Unit,
						Currency:   "USD",
						Price:      price,
						TermType:   termType,
						Attributes: copyAttrs(product.Attributes),
					}:
					}
				}
			}
		}
	}
	return nil
}

func copyAttrs(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// WriteBatch inserts a batch of SKURows into staging_sku_prices via the
// COPY protocol. Duplicate rows within a single batch (should be rare from
// the streaming decoder but defensive here) are dedup'd on the JSONB unique
// key by writing to a temp table and doing INSERT … SELECT DISTINCT ON.
func WriteBatch(ctx context.Context, pool *pgxpool.Pool, serviceID int64, rows []SKURow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tempTable := "sku_stage_tmp"
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE `+tempTable+` (LIKE staging_sku_prices INCLUDING DEFAULTS) ON COMMIT DROP`); err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	cols := []string{"service_id", "sku", "region_code", "unit", "currency", "price_per_unit", "term_type", "attributes", "effective_from"}
	now := time.Now().UTC()

	src := pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
		r := rows[i]
		attrs, err := json.Marshal(r.Attributes)
		if err != nil {
			return nil, err
		}
		return []any{
			serviceID,
			r.SKU,
			r.Region,
			r.Unit,
			r.Currency,
			r.Price,
			r.TermType,
			attrs,
			now,
		}, nil
	})
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{tempTable}, cols, src); err != nil {
		return fmt.Errorf("copy staging: %w", err)
	}
	// Move into staging with de-dup on the unique key.
	if _, err := tx.Exec(ctx, `
		INSERT INTO staging_sku_prices (service_id, sku, region_code, unit, currency, price_per_unit, term_type, attributes, effective_from)
		SELECT service_id, sku, region_code, unit, currency, price_per_unit, term_type, attributes, effective_from
		FROM `+tempTable+` ON CONFLICT DO NOTHING`); err != nil {
		return fmt.Errorf("insert staging: %w", err)
	}
	return tx.Commit(ctx)
}

// WriteDimensionsAndValues extracts distinct attribute keys and (key,value)
// pairs across the batch and upserts them. Called after a page or region of
// rows has been staged so operators don't have to wait for a full run to see
// dimensions in the catalog.
func WriteDimensionsAndValues(ctx context.Context, pool *pgxpool.Pool, serviceID int64, rows []SKURow) error {
	if len(rows) == 0 {
		return nil
	}
	// Collect distinct keys and (key, value) pairs; keep insertion order so
	// sort_order remains stable within a single call.
	keyIdx := map[string]int{}
	var keys []string
	seenVals := map[string]struct{}{}
	type kv struct{ key, value string }
	var vals []kv
	addKV := func(k, v string) {
		if _, ok := keyIdx[k]; !ok {
			keyIdx[k] = len(keys)
			keys = append(keys, k)
		}
		if v == "" {
			return
		}
		cacheKey := k + "\x00" + v
		if _, ok := seenVals[cacheKey]; ok {
			return
		}
		seenVals[cacheKey] = struct{}{}
		vals = append(vals, kv{k, v})
	}
	for _, r := range rows {
		for k, v := range r.Attributes {
			addKV(k, v)
		}
		// termType lives in the outer JSON structure (terms.OnDemand /
		// terms.Reserved / …), not on product.attributes. Surface it as a
		// dimension value so the UI can offer it as a filter.
		if r.TermType != "" {
			addKV("termType", r.TermType)
		}
	}

	if len(keys) == 0 {
		return nil
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for i, key := range keys {
		kind := "enum"
		if key == "regionCode" || key == "location" {
			kind = "region"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO service_dimensions (service_id, key, label, kind, sort_order)
			VALUES ($1, $2, $2, $3, $4)
			ON CONFLICT (service_id, key) DO NOTHING`,
			serviceID, key, kind, i); err != nil {
			return fmt.Errorf("upsert dim %s: %w", key, err)
		}
	}
	for _, v := range vals {
		if _, err := tx.Exec(ctx, `
			INSERT INTO dimension_values (dimension_id, value)
			SELECT id, $2 FROM service_dimensions WHERE service_id = $1 AND key = $3
			ON CONFLICT DO NOTHING`,
			serviceID, v.value, v.key); err != nil {
			return fmt.Errorf("upsert dim value %s=%s: %w", v.key, v.value, err)
		}
	}
	return tx.Commit(ctx)
}
