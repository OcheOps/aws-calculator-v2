# AWS Calculator — Plan 09: Foundation & EC2 Pilot

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the backend + frontend scaffolding for curated per-service forms — migration `0006_curated_lines`, the `line_type`/`service_key`/`user_input_snapshot`/`components_snapshot` round-trip through the estimates store + handlers + repricer + exporters, four shared Svelte primitives with vitest coverage, and EC2 as the pilot curated service (mapper, form, registry). After this plan the app still runs the old UI, but the new pipes and one working curated service are in place and hittable via API.

**Architecture:** Single additive Postgres migration keeps every existing `estimate_items` row valid as `line_type='legacy'`. Curated rows record `service_key`, the user's raw input, and an array of priced components. Reprice/export handle both shapes by branching on `line_type`. On the frontend, a small `services/` package holds one Svelte file + one TypeScript mapper per service; the mapper is a pure function tested with `vitest`, so per-service coverage stays fast and cheap.

**Tech Stack:** Go 1.25 (pgx/v5, chi), Postgres 16, SvelteKit 2 + Svelte 5 runes, Tailwind 4, vitest for pure-function tests, Svelte Testing Library for primitives.

**Spec:** `docs/superpowers/specs/2026-07-04-saas-curated-forms-design.md`

---

## File structure produced by this plan

```
api/
├── migrations/
│   ├── 0006_curated_lines.up.sql        (NEW)
│   └── 0006_curated_lines.down.sql      (NEW)
├── internal/
│   ├── estimates/
│   │   ├── model.go                     (MODIFIED: Item gains LineType/ServiceKey/UserInputSnapshot/ComponentsSnapshot)
│   │   ├── store.go                     (MODIFIED: AddItem accepts curated shape; Get scans new columns)
│   │   ├── store_test.go                (MODIFIED: curated round-trip)
│   │   ├── repricer.go                  (MODIFIED: iterate components for curated lines)
│   │   └── repricer_test.go             (MODIFIED: curated reprice)
│   ├── export/
│   │   ├── csv.go                       (MODIFIED: curated summary)
│   │   ├── json.go                      (unchanged; already dumps struct)
│   │   ├── pdf.go                       (MODIFIED: curated summary)
│   │   └── pdf_test.go                  (MODIFIED)
│   └── httpapi/
│       ├── estimates.go                 (MODIFIED: pass curated fields through)
│       └── estimates_test.go            (MODIFIED: curated CRUD)

web/
├── package.json                         (MODIFIED: add vitest + @testing-library/svelte)
├── vite.config.ts                       (MODIFIED: vitest config)
├── src/
│   ├── lib/
│   │   ├── api/
│   │   │   └── types.ts                 (MODIFIED: PricedComponent + updated EstimateItem)
│   │   ├── components/
│   │   │   ├── Combobox.svelte          (NEW)
│   │   │   ├── Combobox.test.ts         (NEW)
│   │   │   ├── PresetSelect.svelte      (NEW)
│   │   │   ├── PresetSelect.test.ts     (NEW)
│   │   │   ├── NumberInput.svelte       (NEW)
│   │   │   ├── NumberInput.test.ts      (NEW)
│   │   │   ├── RegionPicker.svelte      (NEW)
│   │   │   └── RegionPicker.test.ts     (NEW)
│   │   └── services/
│   │       ├── types.ts                 (NEW: ServiceMeta, PricedComponent, Mapper)
│   │       ├── index.ts                 (NEW: registry)
│   │       └── ec2/
│   │           ├── mapper.ts            (NEW)
│   │           ├── mapper.test.ts       (NEW)
│   │           ├── Form.svelte          (NEW)
│   │           └── icon.svg             (NEW)
```

---

## Task 1: Migration 0006 — curated line columns

**Files:**
- Create: `api/migrations/0006_curated_lines.up.sql`
- Create: `api/migrations/0006_curated_lines.down.sql`

- [ ] **Step 1: Write `0006_curated_lines.up.sql`**

```sql
ALTER TABLE estimate_items
    ADD COLUMN line_type            TEXT NOT NULL DEFAULT 'legacy',
    ADD COLUMN service_key          TEXT,
    ADD COLUMN user_input_snapshot  JSONB,
    ADD COLUMN components_snapshot  JSONB;

CREATE INDEX estimate_items_service_key
    ON estimate_items (service_key)
    WHERE service_key IS NOT NULL;

ALTER TABLE estimate_items
    ADD CONSTRAINT estimate_items_line_type_check
    CHECK (line_type IN ('legacy', 'curated'));
```

- [ ] **Step 2: Write `0006_curated_lines.down.sql`**

```sql
ALTER TABLE estimate_items DROP CONSTRAINT IF EXISTS estimate_items_line_type_check;
DROP INDEX IF EXISTS estimate_items_service_key;
ALTER TABLE estimate_items
    DROP COLUMN IF EXISTS components_snapshot,
    DROP COLUMN IF EXISTS user_input_snapshot,
    DROP COLUMN IF EXISTS service_key,
    DROP COLUMN IF EXISTS line_type;
```

- [ ] **Step 3: Apply + verify**

```bash
docker rm -f pg-plan09 >/dev/null 2>&1
docker run --rm -d --name pg-plan09 -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55270:5432 postgres:16 >/dev/null
for i in $(seq 1 30); do PGPASSWORD=pw psql -h 127.0.0.1 -p 55270 -U u -d d -c "SELECT 1" >/dev/null 2>&1 && break; sleep 1; done
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55270/d?sslmode=disable" go test ./internal/db/... -run TestMigrate -v
cd ..
docker rm -f pg-plan09 >/dev/null
```

Expected: `TestMigrate_AppliesInitialSchema` PASS.

- [ ] **Step 4: Commit**

```bash
git add api/migrations/0006_curated_lines.up.sql api/migrations/0006_curated_lines.down.sql
git commit -m "feat(db): 0006 curated line columns (line_type, service_key, snapshots)"
```

---

## Task 2: Extend `estimates.Item` and `AddItemInput`

**Files:**
- Modify: `api/internal/estimates/model.go`

- [ ] **Step 1: Rewrite `api/internal/estimates/model.go`**

```go
package estimates

import (
	"time"

	"github.com/shopspring/decimal"
)

type Estimate struct {
	ID        int64           `json:"id"`
	OrgID     *int64          `json:"org_id,omitempty"`
	CreatedBy *int64          `json:"created_by,omitempty"`
	Name      string          `json:"name"`
	Currency  string          `json:"currency"`
	Notes     string          `json:"notes,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Items     []Item          `json:"items,omitempty"`
	Total     decimal.Decimal `json:"total,omitempty"`
}

// LineType distinguishes the two shapes an estimate row can take.
// Legacy rows carry a single SKU price directly on the row; curated rows
// carry an array of priced components + the user's original input so the
// UI can rebuild the human summary.
type LineType string

const (
	LineTypeLegacy  LineType = "legacy"
	LineTypeCurated LineType = "curated"
)

type Component struct {
	SKU         string          `json:"sku"`
	Unit        string          `json:"unit"`
	Quantity    decimal.Decimal `json:"quantity"`
	UnitPrice   decimal.Decimal `json:"unit_price"`
	MonthlyCost decimal.Decimal `json:"monthly_cost"`
	Currency    string          `json:"currency"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Region      string          `json:"region_code,omitempty"`
	Service     string          `json:"service_code,omitempty"`
}

type Item struct {
	ID                  int64             `json:"id"`
	EstimateID          int64             `json:"estimate_id"`
	LineType            LineType          `json:"line_type"`
	ServiceKey          string            `json:"service_key,omitempty"`
	ServiceID           int64             `json:"service_id"`
	ServiceCode         string            `json:"service_code,omitempty"`
	RegionCode          string            `json:"region_code"`
	SKU                 string            `json:"sku"`
	Quantity            decimal.Decimal   `json:"quantity"`
	Attributes          map[string]string `json:"attributes"`
	UnitPriceSnapshot   decimal.Decimal   `json:"unit_price"`
	CurrencySnapshot    string            `json:"currency"`
	MonthlyCostSnapshot decimal.Decimal   `json:"monthly_cost"`
	UserInputSnapshot   any               `json:"user_input,omitempty"`
	ComponentsSnapshot  []Component       `json:"components,omitempty"`
}

const HoursPerMonth = 730
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./... && cd ..
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add api/internal/estimates/model.go
git commit -m "feat(estimates): Item + Component model for curated lines"
```

---

## Task 3: `AddItem` and `Get` handle both shapes

**Files:**
- Modify: `api/internal/estimates/store.go`

The store gains a curated variant on `AddItem` and its `Get` scans the new columns.

- [ ] **Step 1: Add curated input struct and helper at the top of `store.go`**

Insert after the existing `AddItemInput` type:

```go
// AddCuratedItemInput is used by curated services (EC2, RDS, S3, …). The
// caller supplies the resolved priced components; the store persists them
// verbatim and sums monthly_cost.
type AddCuratedItemInput struct {
	ServiceID         int64
	ServiceKey        string // 'ec2', 'rds', …
	RegionCode        string
	SyntheticSKU      string // 'curated:ec2:v1'
	Currency          string
	UserInputSnapshot any
	Components        []Component
}
```

Then add the new method (put it under `AddItem`):

```go
// AddCuratedItem inserts a curated line whose price is the sum of N
// components. It records the raw user input for later reprice + export.
func (s *Store) AddCuratedItem(ctx context.Context, estimateID int64, in AddCuratedItemInput) (*Item, error) {
	if len(in.Components) == 0 {
		return nil, fmt.Errorf("at least one component required")
	}
	monthly := decimal.NewFromInt(0)
	for _, c := range in.Components {
		monthly = monthly.Add(c.MonthlyCost)
	}
	userInput, err := json.Marshal(in.UserInputSnapshot)
	if err != nil {
		return nil, err
	}
	components, err := json.Marshal(in.Components)
	if err != nil {
		return nil, err
	}
	first := in.Components[0]
	var item Item
	err = s.pool.QueryRow(ctx, `
		INSERT INTO estimate_items (
			estimate_id, service_id, region_code, sku, quantity, unit_override,
			attributes_snapshot, unit_price_snapshot, currency_snapshot,
			monthly_cost_snapshot,
			line_type, service_key, user_input_snapshot, components_snapshot
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'curated', $11, $12, $13)
		RETURNING id`,
		estimateID, in.ServiceID, in.RegionCode, in.SyntheticSKU,
		first.Quantity.String(), first.Unit,
		mustJSON(first.Attributes),
		first.UnitPrice.String(), in.Currency, monthly.String(),
		in.ServiceKey, userInput, components).Scan(&item.ID)
	if err != nil {
		return nil, err
	}
	item.EstimateID = estimateID
	item.LineType = LineTypeCurated
	item.ServiceID = in.ServiceID
	item.ServiceKey = in.ServiceKey
	item.RegionCode = in.RegionCode
	item.SKU = in.SyntheticSKU
	item.CurrencySnapshot = in.Currency
	item.MonthlyCostSnapshot = monthly
	item.UnitPriceSnapshot = first.UnitPrice
	item.Quantity = first.Quantity
	item.Attributes = first.Attributes
	item.UserInputSnapshot = in.UserInputSnapshot
	item.ComponentsSnapshot = in.Components
	if _, err := s.pool.Exec(ctx, `UPDATE estimates SET updated_at = NOW() WHERE id = $1`, estimateID); err != nil {
		return nil, err
	}
	return &item, nil
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}
```

- [ ] **Step 2: Update `Get` to scan the new columns**

Find the `Get` method (around 40 lines from the top of `store.go`). Replace it with:

```go
func (s *Store) Get(ctx context.Context, id int64) (*Estimate, error) {
	var est Estimate
	err := s.pool.QueryRow(ctx, `
		SELECT id, org_id, created_by, name, currency, COALESCE(notes,''), created_at, updated_at
		FROM estimates WHERE id = $1`, id).Scan(&est.ID, &est.OrgID, &est.CreatedBy, &est.Name, &est.Currency, &est.Notes, &est.CreatedAt, &est.UpdatedAt)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT ei.id, ei.estimate_id, ei.service_id, s.code, ei.region_code,
		       ei.sku, ei.quantity::text, ei.attributes_snapshot,
		       ei.unit_price_snapshot::text, ei.currency_snapshot,
		       ei.monthly_cost_snapshot::text,
		       ei.line_type, COALESCE(ei.service_key,''),
		       ei.user_input_snapshot, ei.components_snapshot
		FROM estimate_items ei JOIN services s ON s.id = ei.service_id
		WHERE ei.estimate_id = $1 ORDER BY ei.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	total := decimal.NewFromInt(0)
	for rows.Next() {
		var it Item
		var qty, price, monthly string
		var attrs []byte
		var lineType string
		var userInput, components []byte
		if err := rows.Scan(
			&it.ID, &it.EstimateID, &it.ServiceID, &it.ServiceCode, &it.RegionCode,
			&it.SKU, &qty, &attrs, &price, &it.CurrencySnapshot, &monthly,
			&lineType, &it.ServiceKey, &userInput, &components,
		); err != nil {
			return nil, err
		}
		it.LineType = LineType(lineType)
		it.Quantity, _ = decimal.NewFromString(qty)
		it.UnitPriceSnapshot, _ = decimal.NewFromString(price)
		it.MonthlyCostSnapshot, _ = decimal.NewFromString(monthly)
		_ = json.Unmarshal(attrs, &it.Attributes)
		if len(userInput) > 0 && string(userInput) != "null" {
			_ = json.Unmarshal(userInput, &it.UserInputSnapshot)
		}
		if len(components) > 0 && string(components) != "null" {
			_ = json.Unmarshal(components, &it.ComponentsSnapshot)
		}
		est.Items = append(est.Items, it)
		total = total.Add(it.MonthlyCostSnapshot)
	}
	est.Total = total
	return &est, nil
}
```

- [ ] **Step 3: Update `AddItem` (the legacy path) to set `line_type = 'legacy'` explicitly**

Find the existing `AddItem` method. The `INSERT` becomes:

```go
	err = s.pool.QueryRow(ctx, `
		INSERT INTO estimate_items (
			estimate_id, service_id, region_code, sku, quantity, unit_override,
			attributes_snapshot, unit_price_snapshot, currency_snapshot,
			monthly_cost_snapshot, line_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'legacy')
		RETURNING id`,
		estimateID, in.ServiceID, in.RegionCode, in.SKU, in.Quantity.String(), in.Unit, attrs, in.UnitPrice.String(), in.Currency, monthly.String()).
		Scan(&item.ID)
```

After the Scan, set `item.LineType = LineTypeLegacy` before returning.

- [ ] **Step 4: Verify build**

```bash
cd api && go build ./... && cd ..
```

- [ ] **Step 5: Commit**

```bash
git add api/internal/estimates/store.go
git commit -m "feat(estimates): AddCuratedItem + Get round-trips new columns"
```

---

## Task 4: Store test for curated round-trip

**Files:**
- Modify: `api/internal/estimates/store_test.go`

- [ ] **Step 1: Add a test at the bottom of `store_test.go`**

```go
func TestStore_AddCuratedItemRoundTrip(t *testing.T) {
	s, svcID, orgID, ctx := seed(t)
	est, _, _ := s.Create(ctx, CreateInput{Name: "curated", Currency: "USD", OrgID: &orgID})

	input := map[string]any{
		"instanceType":  "m5.large",
		"operatingSystem": "Linux",
		"count":         float64(2),
		"hoursPerMonth": float64(730),
	}
	comps := []Component{
		{
			SKU: "EC2-COMPUTE-SKU", Unit: "Hrs",
			Quantity:    decimal.NewFromInt(1460),
			UnitPrice:   decimal.NewFromFloat(0.096),
			MonthlyCost: decimal.NewFromFloat(140.16),
			Currency:    "USD",
			Attributes:  map[string]string{"instanceType": "m5.large"},
			Region:      "us-east-1",
			Service:     "AmazonEC2",
		},
	}
	item, err := s.AddCuratedItem(ctx, est.ID, AddCuratedItemInput{
		ServiceID:         svcID,
		ServiceKey:        "ec2",
		RegionCode:        "us-east-1",
		SyntheticSKU:      "curated:ec2:v1",
		Currency:          "USD",
		UserInputSnapshot: input,
		Components:        comps,
	})
	if err != nil {
		t.Fatalf("AddCuratedItem: %v", err)
	}
	if item.LineType != LineTypeCurated || item.ServiceKey != "ec2" {
		t.Errorf("wrong line metadata: %+v", item)
	}
	if !item.MonthlyCostSnapshot.Equal(decimal.NewFromFloat(140.16)) {
		t.Errorf("monthly = %s, want 140.16", item.MonthlyCostSnapshot)
	}

	got, err := s.Get(ctx, est.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(got.Items))
	}
	back := got.Items[0]
	if back.LineType != LineTypeCurated {
		t.Errorf("LineType = %q", back.LineType)
	}
	if back.ServiceKey != "ec2" {
		t.Errorf("ServiceKey = %q", back.ServiceKey)
	}
	if len(back.ComponentsSnapshot) != 1 || back.ComponentsSnapshot[0].SKU != "EC2-COMPUTE-SKU" {
		t.Errorf("components lost: %+v", back.ComponentsSnapshot)
	}
	if back.UserInputSnapshot == nil {
		t.Errorf("user input snapshot missing")
	}
}
```

- [ ] **Step 2: Run the test**

```bash
docker rm -f pg-p09b >/dev/null 2>&1
docker run --rm -d --name pg-p09b -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55272:5432 postgres:16 >/dev/null
for i in $(seq 1 30); do PGPASSWORD=pw psql -h 127.0.0.1 -p 55272 -U u -d d -c "SELECT 1" >/dev/null 2>&1 && break; sleep 1; done
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55272/d?sslmode=disable" go test ./internal/estimates/... -v -run TestStore_AddCuratedItemRoundTrip
cd ..
docker rm -f pg-p09b >/dev/null
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add api/internal/estimates/store_test.go
git commit -m "test(estimates): curated line round-trip"
```

---

## Task 5: Repricer branches on line type

**Files:**
- Modify: `api/internal/estimates/repricer.go`
- Modify: `api/internal/estimates/repricer_test.go`

The current repricer builds one lookup per item from `attributes_snapshot`. Curated lines carry N components; we reprice each one and recompute the sum. The `PriceDelta` returned to the client still has one entry per item — the sums.

- [ ] **Step 1: Rewrite `Preview` in `repricer.go`**

Replace the whole `Preview` method:

```go
func (r *Repricer) Preview(ctx context.Context, estimateID int64) ([]PriceDelta, error) {
	est, err := r.store.Get(ctx, estimateID)
	if err != nil {
		return nil, err
	}
	deltas := make([]PriceDelta, 0, len(est.Items))
	for _, it := range est.Items {
		newMonthly := decimal.NewFromInt(0)
		newUnit := decimal.NewFromInt(0)
		currency := it.CurrencySnapshot

		if it.LineType == LineTypeCurated && len(it.ComponentsSnapshot) > 0 {
			ok := true
			for _, c := range it.ComponentsSnapshot {
				res, err := r.lookup.Price(ctx, pricing.LookupInput{
					ServiceCode: c.Service,
					Region:      c.Region,
					Attributes:  c.Attributes,
					Currency:    currency,
					TermType:    "OnDemand",
				})
				if err != nil {
					ok = false
					break
				}
				monthly := MonthlyCost(res.Unit, res.PricePerUnit, c.Quantity)
				newMonthly = newMonthly.Add(monthly)
				newUnit = res.PricePerUnit
				currency = res.Currency
			}
			if !ok {
				continue
			}
		} else {
			res, err := r.lookup.Price(ctx, pricing.LookupInput{
				ServiceCode: it.ServiceCode,
				Region:      it.RegionCode,
				Attributes:  it.Attributes,
				Currency:    currency,
				TermType:    "OnDemand",
			})
			if err != nil {
				continue
			}
			newUnit = res.PricePerUnit
			newMonthly = MonthlyCost(res.Unit, res.PricePerUnit, it.Quantity)
			currency = res.Currency
		}

		deltas = append(deltas, PriceDelta{
			ItemID:       it.ID,
			OldUnitPrice: it.UnitPriceSnapshot,
			NewUnitPrice: newUnit,
			OldMonthly:   it.MonthlyCostSnapshot,
			NewMonthly:   newMonthly,
			Currency:     currency,
		})
	}
	return deltas, nil
}
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./... && cd ..
```

- [ ] **Step 3: Add a test that seeds a curated line, changes the underlying sku_prices row, and asserts the delta**

At the bottom of `repricer_test.go`:

```go
func TestRepricer_CuratedLineSumsComponents(t *testing.T) {
	// Fresh setup: two SKUs (compute + storage) so a curated 2-component
	// line has something to sum.
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(pool.Close)
	_, _ = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var providerID, ec2ID int64
	pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1,'AmazonEC2','EC2') RETURNING id`, providerID).Scan(&ec2ID)
	pool.Exec(ctx, `INSERT INTO service_dimensions (service_id, key, label, kind) VALUES ($1,'instanceType','','enum')`, ec2ID)
	pool.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type, attributes)
		VALUES ($1,'EC2-COMPUTE','us-east-1','Hrs',0.096,'OnDemand','{"instanceType":"m5.large"}'::jsonb),
		       ($1,'EBS-STORE','us-east-1','GB-Mo',0.10,'OnDemand','{"volumeType":"gp3"}'::jsonb)`, ec2ID)

	cache, _ := pricing.NewCache(1<<10, time.Minute)
	lookup := pricing.NewLookup(pricing.NewStore(pool), fx.NewStore(pool), cache, catalog.NewStore(pool))
	store := NewStore(pool)

	var orgID int64
	pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('t','t') RETURNING id`).Scan(&orgID)
	est, _, _ := store.Create(ctx, CreateInput{Name: "x", Currency: "USD", OrgID: &orgID})
	_, err = store.AddCuratedItem(ctx, est.ID, AddCuratedItemInput{
		ServiceID:    ec2ID,
		ServiceKey:   "ec2",
		RegionCode:   "us-east-1",
		SyntheticSKU: "curated:ec2:v1",
		Currency:     "USD",
		UserInputSnapshot: map[string]any{"instanceType": "m5.large"},
		Components: []Component{
			{
				SKU: "EC2-COMPUTE", Unit: "Hrs",
				Quantity: decimal.NewFromInt(730), UnitPrice: decimal.NewFromFloat(0.096),
				MonthlyCost: decimal.NewFromFloat(70.08), Currency: "USD",
				Attributes: map[string]string{"instanceType": "m5.large"},
				Region:     "us-east-1", Service: "AmazonEC2",
			},
			{
				SKU: "EBS-STORE", Unit: "GB-Mo",
				Quantity: decimal.NewFromInt(100), UnitPrice: decimal.NewFromFloat(0.10),
				MonthlyCost: decimal.NewFromFloat(10.0), Currency: "USD",
				Attributes: map[string]string{"volumeType": "gp3"},
				Region:     "us-east-1", Service: "AmazonEC2",
			},
		},
	})
	if err != nil {
		t.Fatalf("AddCuratedItem: %v", err)
	}

	// Mutate the underlying compute price.
	pool.Exec(ctx, `UPDATE sku_prices SET price_per_unit = 0.192 WHERE sku='EC2-COMPUTE'`)

	rep := NewRepricer(store, lookup)
	deltas, err := rep.Preview(ctx, est.ID)
	if err != nil || len(deltas) != 1 {
		t.Fatalf("Preview: err=%v deltas=%d", err, len(deltas))
	}
	// New monthly = 0.192 * 730 + 0.10 * 100 = 140.16 + 10 = 150.16
	if !deltas[0].NewMonthly.Equal(decimal.NewFromFloat(150.16)) {
		t.Errorf("new monthly = %s, want 150.16", deltas[0].NewMonthly)
	}
	if !deltas[0].OldMonthly.Equal(decimal.NewFromFloat(80.08)) {
		t.Errorf("old monthly = %s, want 80.08", deltas[0].OldMonthly)
	}
}
```

- [ ] **Step 4: Run the test**

```bash
docker rm -f pg-p09c >/dev/null 2>&1
docker run --rm -d --name pg-p09c -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55274:5432 postgres:16 >/dev/null
for i in $(seq 1 30); do PGPASSWORD=pw psql -h 127.0.0.1 -p 55274 -U u -d d -c "SELECT 1" >/dev/null 2>&1 && break; sleep 1; done
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55274/d?sslmode=disable" go test ./internal/estimates/... -v -run Curated
cd ..
docker rm -f pg-p09c >/dev/null
```

Expected: both curated tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/estimates/repricer.go api/internal/estimates/repricer_test.go
git commit -m "feat(estimates): reprice sums components for curated lines"
```

---

## Task 6: Export renders curated summaries

**Files:**
- Modify: `api/internal/export/csv.go`
- Modify: `api/internal/export/pdf.go`
- Modify: `api/internal/export/pdf_test.go`

For a curated row the summary should read like "EC2 · 2 × m5.large in us-east-1 · 730 h/mo". Simplest and provider-agnostic approach: use `service_key` + a small formatter per key; fall back to service code + first-component attributes if the key is unknown.

- [ ] **Step 1: Add a summary helper**

Create `api/internal/export/summary.go`:

```go
package export

import (
	"fmt"
	"sort"
	"strings"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
)

// LineSummary returns a human-readable string for the estimate item, used by
// CSV/PDF exporters. Curated lines get service-specific formatting; legacy
// falls back to attribute list.
func LineSummary(it estimates.Item) string {
	switch it.LineType {
	case estimates.LineTypeCurated:
		if s := curatedSummary(it); s != "" {
			return s
		}
	}
	return legacySummary(it)
}

func curatedSummary(it estimates.Item) string {
	m, ok := it.UserInputSnapshot.(map[string]any)
	if !ok {
		return ""
	}
	switch it.ServiceKey {
	case "ec2":
		return fmt.Sprintf("EC2 · %v × %v in %s · %v h/mo · %v",
			m["count"], m["instanceType"], it.RegionCode, m["hoursPerMonth"], m["operatingSystem"])
	}
	// Generic curated fallback: sorted key=value list.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, m[k]))
	}
	return fmt.Sprintf("%s · %s · %s", it.ServiceKey, it.RegionCode, strings.Join(pairs, ", "))
}

func legacySummary(it estimates.Item) string {
	keys := make([]string, 0, len(it.Attributes))
	for k := range it.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, it.Attributes[k]))
	}
	return fmt.Sprintf("%s · %s · %s", it.ServiceCode, it.RegionCode, strings.Join(pairs, ", "))
}
```

- [ ] **Step 2: Update `csv.go`** so each row uses `LineSummary` in a new column

Find the existing header write. Replace the header + row loop:

```go
	if err := c.Write([]string{"service", "region", "sku", "quantity", "unit_price", "monthly_cost", "currency", "summary"}); err != nil {
		return err
	}
	for _, it := range est.Items {
		if err := c.Write([]string{
			it.ServiceCode, it.RegionCode, it.SKU,
			it.Quantity.String(), it.UnitPriceSnapshot.String(),
			it.MonthlyCostSnapshot.String(), it.CurrencySnapshot,
			LineSummary(it),
		}); err != nil {
			return err
		}
	}
```

- [ ] **Step 3: Update `pdf.go`** so item rows use `LineSummary`

Find the per-item cell that currently writes `it.ServiceCode`. Replace the 60-mm cell content with `LineSummary(it)`. Keep the same widths.

Concretely:

```go
	for _, it := range est.Items {
		pdf.CellFormat(90, 7, LineSummary(it), "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 7, it.RegionCode, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 7, it.Quantity.String(), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 7, it.MonthlyCostSnapshot.StringFixed(4), "1", 1, "R", false, 0, "")
	}
```

Adjust the table header row widths to match (`Item`/`Region`/`Qty`/`Monthly`):

```go
	pdf.CellFormat(90, 8, "Item", "1", 0, "L", false, 0, "")
	pdf.CellFormat(30, 8, "Region", "1", 0, "L", false, 0, "")
	pdf.CellFormat(20, 8, "Qty", "1", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, "Monthly", "1", 1, "R", false, 0, "")
```

Also change the Total row to span 140mm to align:

```go
	pdf.CellFormat(140, 8, "Total", "1", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, est.Total.StringFixed(4)+" "+est.Currency, "1", 1, "R", false, 0, "")
```

- [ ] **Step 4: Extend `pdf_test.go`** with a curated case

Add:

```go
func TestExport_CuratedSummary(t *testing.T) {
	est := &estimates.Estimate{
		Name: "curated", Currency: "USD", Total: decimal.NewFromFloat(140.16),
		Items: []estimates.Item{{
			LineType:    estimates.LineTypeCurated,
			ServiceKey:  "ec2",
			ServiceCode: "AmazonEC2",
			RegionCode:  "us-east-1",
			SKU:         "curated:ec2:v1",
			Quantity:    decimal.NewFromInt(1460),
			UnitPriceSnapshot: decimal.NewFromFloat(0.096),
			MonthlyCostSnapshot: decimal.NewFromFloat(140.16),
			CurrencySnapshot: "USD",
			UserInputSnapshot: map[string]any{
				"count": float64(2), "instanceType": "m5.large",
				"hoursPerMonth": float64(730), "operatingSystem": "Linux",
			},
		}},
	}
	got := LineSummary(est.Items[0])
	if !strings.Contains(got, "EC2") || !strings.Contains(got, "m5.large") || !strings.Contains(got, "Linux") {
		t.Errorf("summary = %q", got)
	}
}
```

Add `import "strings"` at the top of `pdf_test.go` if not present.

- [ ] **Step 5: Run tests**

```bash
cd api && go test ./internal/export/... -v && cd ..
```

Expected: all export tests PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/export/
git commit -m "feat(export): human summary for curated lines (CSV + PDF)"
```

---

## Task 7: HTTP handler accepts curated items

**Files:**
- Modify: `api/internal/httpapi/estimates.go`
- Modify: `api/internal/httpapi/estimates_test.go`

- [ ] **Step 1: Update the `POST /estimates/:id/items` handler**

Find the `addItem` function in `estimates.go`. Replace the body-parse + call block:

```go
		var body struct {
			LineType   string            `json:"line_type"`
			ServiceID  int64             `json:"service_id"`
			ServiceKey string            `json:"service_key"`
			RegionCode string            `json:"region_code"`
			SKU        string            `json:"sku"`
			Quantity   string            `json:"quantity"`
			Attributes map[string]string `json:"attributes"`
			UnitPrice  string            `json:"unit_price"`
			Unit       string            `json:"unit"`
			Currency   string            `json:"currency"`
			// Curated-only:
			UserInput  json.RawMessage   `json:"user_input"`
			Components []struct {
				Service     string            `json:"service_code"`
				Region      string            `json:"region_code"`
				SKU         string            `json:"sku"`
				Unit        string            `json:"unit"`
				Quantity    string            `json:"quantity"`
				UnitPrice   string            `json:"unit_price"`
				MonthlyCost string            `json:"monthly_cost"`
				Currency    string            `json:"currency"`
				Attributes  map[string]string `json:"attributes"`
			} `json:"components"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if body.LineType == "curated" {
			var userInput any
			_ = json.Unmarshal(body.UserInput, &userInput)
			comps := make([]estimates.Component, 0, len(body.Components))
			for _, c := range body.Components {
				qty, _ := decimal.NewFromString(c.Quantity)
				up, _ := decimal.NewFromString(c.UnitPrice)
				m, _ := decimal.NewFromString(c.MonthlyCost)
				comps = append(comps, estimates.Component{
					SKU: c.SKU, Unit: c.Unit,
					Quantity: qty, UnitPrice: up, MonthlyCost: m,
					Currency:   c.Currency,
					Attributes: c.Attributes,
					Region:     c.Region,
					Service:    c.Service,
				})
			}
			item, err := d.Store.AddCuratedItem(r.Context(), id, estimates.AddCuratedItemInput{
				ServiceID:         body.ServiceID,
				ServiceKey:        body.ServiceKey,
				RegionCode:        body.RegionCode,
				SyntheticSKU:      body.SKU,
				Currency:          body.Currency,
				UserInputSnapshot: userInput,
				Components:        comps,
			})
			if err != nil {
				WriteError(w, http.StatusInternalServerError, "internal", err.Error())
				return
			}
			WriteData(w, http.StatusCreated, item, nil)
			return
		}
		// legacy path (unchanged)
		qty, _ := decimal.NewFromString(body.Quantity)
		price, _ := decimal.NewFromString(body.UnitPrice)
		item, err := d.Store.AddItem(r.Context(), id, estimates.AddItemInput{
			ServiceID: body.ServiceID, RegionCode: body.RegionCode, SKU: body.SKU,
			Quantity: qty, Attributes: body.Attributes,
			UnitPrice: price, Unit: body.Unit, Currency: body.Currency,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusCreated, item, nil)
```

- [ ] **Step 2: Add a handler test for the curated shape**

At the end of `estimates_test.go`:

```go
func TestEstimates_CuratedItemRoundTrip(t *testing.T) {
	srv, svcID := setupEstimateHandlers(t)
	defer srv.Close()
	c := &http.Client{}

	body, _ := json.Marshal(map[string]any{"name": "curated", "currency": "USD"})
	resp, _ := c.Post(srv.URL+"/api/v1/estimates", "application/json", bytes.NewReader(body))
	var created struct {
		Data struct {
			Estimate  estimates.Estimate `json:"estimate"`
			EditToken string             `json:"edit_token"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	id := created.Data.Estimate.ID

	item := map[string]any{
		"line_type":   "curated",
		"service_id":  svcID,
		"service_key": "ec2",
		"region_code": "us-east-1",
		"sku":         "curated:ec2:v1",
		"currency":    "USD",
		"user_input":  map[string]any{"instanceType": "m5.large", "count": 2},
		"components": []map[string]any{{
			"service_code": "AmazonEC2",
			"region_code":  "us-east-1",
			"sku":          "EC2-COMPUTE",
			"unit":         "Hrs",
			"quantity":     "1460",
			"unit_price":   "0.096",
			"monthly_cost": "140.16",
			"currency":     "USD",
			"attributes":   map[string]string{"instanceType": "m5.large"},
		}},
	}
	buf, _ := json.Marshal(item)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/estimates/"+itoa(id)+"/items", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Edit-Token", created.Data.EditToken)
	resp, err := c.Do(req)
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("post curated status=%v err=%v", resp.StatusCode, err)
	}
	resp.Body.Close()

	req, _ = http.NewRequest("GET", srv.URL+"/api/v1/estimates/"+itoa(id), nil)
	req.Header.Set("X-Edit-Token", created.Data.EditToken)
	resp, _ = c.Do(req)
	var got struct{ Data estimates.Estimate }
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if len(got.Data.Items) != 1 {
		t.Fatalf("items = %d", len(got.Data.Items))
	}
	if got.Data.Items[0].LineType != estimates.LineTypeCurated {
		t.Errorf("LineType = %q", got.Data.Items[0].LineType)
	}
	if len(got.Data.Items[0].ComponentsSnapshot) != 1 {
		t.Errorf("components = %d", len(got.Data.Items[0].ComponentsSnapshot))
	}
}
```

- [ ] **Step 3: Run the test**

```bash
docker rm -f pg-p09d >/dev/null 2>&1
docker run --rm -d --name pg-p09d -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55276:5432 postgres:16 >/dev/null
for i in $(seq 1 30); do PGPASSWORD=pw psql -h 127.0.0.1 -p 55276 -U u -d d -c "SELECT 1" >/dev/null 2>&1 && break; sleep 1; done
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55276/d?sslmode=disable" go test ./internal/httpapi/... -v -run Curated
cd ..
docker rm -f pg-p09d >/dev/null
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add api/internal/httpapi/estimates.go api/internal/httpapi/estimates_test.go
git commit -m "feat(httpapi): POST /estimates/:id/items accepts curated shape"
```

---

## Task 8: Full backend sweep + push

- [ ] **Step 1: Run the full suite**

```bash
docker rm -f pg-p09e >/dev/null 2>&1
docker run --rm -d --name pg-p09e -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55278:5432 postgres:16 >/dev/null
for i in $(seq 1 30); do PGPASSWORD=pw psql -h 127.0.0.1 -p 55278 -U u -d d -c "SELECT 1" >/dev/null 2>&1 && break; sleep 1; done
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55278/d?sslmode=disable" go test -p 1 -count=1 ./... 2>&1 | tail -20
cd ..
docker rm -f pg-p09e >/dev/null
```

Expected: all packages PASS.

- [ ] **Step 2: Push**

```bash
git push origin main
```

---

## Task 9: Frontend types + vitest setup

**Files:**
- Modify: `web/package.json`
- Modify: `web/vite.config.ts`
- Modify: `web/src/lib/api/types.ts`
- Create: `web/src/lib/services/types.ts`
- Create: `web/vitest.setup.ts`

- [ ] **Step 1: Add vitest + Svelte Testing Library**

```bash
cd web
npm install --save-dev vitest @testing-library/svelte @testing-library/jest-dom @vitest/ui jsdom
cd ..
```

- [ ] **Step 2: Extend `vite.config.ts` with a test block**

Replace `web/vite.config.ts` with:

```typescript
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	test: {
		globals: true,
		environment: 'jsdom',
		setupFiles: ['./vitest.setup.ts'],
		include: ['src/**/*.{test,spec}.{js,ts}']
	}
});
```

- [ ] **Step 3: Create `vitest.setup.ts`**

```typescript
import '@testing-library/jest-dom/vitest';
```

- [ ] **Step 4: Add a `test` script to `web/package.json`**

Under `"scripts"`:

```json
"test": "vitest run",
"test:watch": "vitest"
```

- [ ] **Step 5: Extend `web/src/lib/api/types.ts`**

Add near the `EstimateItem` type:

```ts
export type PricedComponent = {
	service_code: string;
	region_code: string;
	sku: string;
	unit: string;
	quantity: string;
	unit_price: string;
	monthly_cost: string;
	currency: string;
	attributes: Record<string, string>;
};

export type EstimateItem = {
	id: number;
	estimate_id: number;
	service_id: number;
	service_code?: string;
	region_code: string;
	sku: string;
	quantity: string;
	attributes: Record<string, string>;
	unit_price: string;
	currency: string;
	monthly_cost: string;
	line_type: 'legacy' | 'curated';
	service_key?: string;
	user_input?: unknown;
	components?: PricedComponent[];
};
```

Replace the existing `EstimateItem` definition with the new one.

- [ ] **Step 6: Create `web/src/lib/services/types.ts`**

```ts
import type { Component } from 'svelte';

export type PricedComponent = {
	service: string;
	region?: string;
	attributes: Record<string, string>;
	quantity: number;
	unitLabel: string;
	summary?: string;
};

export type ServiceMeta = {
	code: string;
	label: string;
	description: string;
	icon: string;
	category: 'Compute' | 'Storage' | 'Database' | 'Network' | 'Serverless';
};

export interface ServiceMapper<Input> {
	meta: ServiceMeta;
	defaults: Input;
	map(input: Input, region: string): PricedComponent[];
}

export interface ServiceEntry<Input = unknown> {
	meta: ServiceMeta;
	defaults: Input;
	map(input: Input, region: string): PricedComponent[];
	Form: Component;
}
```

- [ ] **Step 7: Run type-check**

```bash
cd web && npm run check 2>&1 | tail -5 && cd ..
```

Expected: 0 errors.

- [ ] **Step 8: Commit**

```bash
git add web/package.json web/package-lock.json web/vite.config.ts web/vitest.setup.ts web/src/lib/api/types.ts web/src/lib/services/types.ts
git commit -m "chore(web): vitest wiring + PricedComponent + ServiceMapper types"
```

---

## Task 10: Shared primitive — `NumberInput`

**Files:**
- Create: `web/src/lib/components/NumberInput.svelte`
- Create: `web/src/lib/components/NumberInput.test.ts`

- [ ] **Step 1: Write `NumberInput.svelte`**

```svelte
<script lang="ts">
	type Props = {
		value?: number;
		label?: string;
		unit?: string;
		min?: number;
		max?: number;
		step?: number;
		placeholder?: string;
	};
	let {
		value = $bindable(0),
		label = '',
		unit = '',
		min,
		max,
		step = 1,
		placeholder = ''
	}: Props = $props();
</script>

<label class="block">
	{#if label}<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>{/if}
	<div class="flex items-center gap-2 rounded-md border border-[--color-border] bg-transparent px-3 py-2 text-sm text-[--color-text] focus-within:border-[--color-text]">
		<input
			type="number"
			bind:value
			{min}
			{max}
			{step}
			{placeholder}
			class="w-full bg-transparent outline-none"
			aria-label={label}
		/>
		{#if unit}<span class="mono text-xs text-[--color-text-muted]">{unit}</span>{/if}
	</div>
</label>
```

- [ ] **Step 2: Write `NumberInput.test.ts`**

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import NumberInput from './NumberInput.svelte';

describe('NumberInput', () => {
	it('renders label + unit', () => {
		render(NumberInput, { props: { label: 'Instances', unit: 'count', value: 3 } });
		expect(screen.getByText('Instances')).toBeInTheDocument();
		expect(screen.getByText('count')).toBeInTheDocument();
		expect(screen.getByRole('spinbutton', { name: 'Instances' })).toHaveValue(3);
	});
});
```

- [ ] **Step 3: Run tests**

```bash
cd web && npm run test 2>&1 | tail -10 && cd ..
```

Expected: 1 test PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/components/NumberInput.svelte web/src/lib/components/NumberInput.test.ts
git commit -m "feat(web): NumberInput primitive with inline unit label"
```

---

## Task 11: Shared primitive — `PresetSelect`

**Files:**
- Create: `web/src/lib/components/PresetSelect.svelte`
- Create: `web/src/lib/components/PresetSelect.test.ts`

- [ ] **Step 1: Write `PresetSelect.svelte`**

```svelte
<script lang="ts" generics="T">
	type Preset = { label: string; value: T };
	type Props = {
		value?: T | undefined;
		presets: Preset[];
		label?: string;
		custom?: { label: string; onSelect: () => void };
	};
	let { value = $bindable(undefined as T | undefined), presets, label = '', custom }: Props = $props();
</script>

<div>
	{#if label}<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>{/if}
	<div class="flex flex-wrap gap-2">
		{#each presets as p (p.label)}
			<button
				type="button"
				onclick={() => (value = p.value)}
				class="rounded-md border px-3 py-1.5 text-xs {value === p.value
					? 'border-[--color-text] bg-[--color-surface-elev] text-[--color-text]'
					: 'border-[--color-border] text-[--color-text-muted] hover:text-[--color-text]'}"
			>
				{p.label}
			</button>
		{/each}
		{#if custom}
			<button
				type="button"
				onclick={() => custom.onSelect()}
				class="rounded-md border border-dashed border-[--color-border] px-3 py-1.5 text-xs text-[--color-text-muted] hover:text-[--color-text]"
			>
				{custom.label}
			</button>
		{/if}
	</div>
</div>
```

- [ ] **Step 2: Write `PresetSelect.test.ts`**

```ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import PresetSelect from './PresetSelect.svelte';

describe('PresetSelect', () => {
	it('selects a preset on click', async () => {
		let selected: number | undefined;
		const { component } = render(PresetSelect, {
			props: {
				presets: [
					{ label: '30 days', value: 720 },
					{ label: 'Full month', value: 730 }
				],
				value: undefined
			}
		});
		// bind:value isn't directly assertable from outside; check that clicking
		// applies the visual selected state.
		const full = screen.getByText('Full month');
		await fireEvent.click(full);
		expect(full.className).toContain('border-[--color-text]');
	});
});
```

- [ ] **Step 3: Run tests**

```bash
cd web && npm run test 2>&1 | tail -10 && cd ..
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/components/PresetSelect.svelte web/src/lib/components/PresetSelect.test.ts
git commit -m "feat(web): PresetSelect primitive (radio buttons + custom escape)"
```

---

## Task 12: Shared primitive — `Combobox`

**Files:**
- Create: `web/src/lib/components/Combobox.svelte`
- Create: `web/src/lib/components/Combobox.test.ts`

- [ ] **Step 1: Write `Combobox.svelte`**

```svelte
<script lang="ts">
	type Option = { value: string; label?: string; group?: string };
	type Props = {
		value?: string;
		options: Option[];
		label?: string;
		placeholder?: string;
	};
	let {
		value = $bindable(''),
		options,
		label = '',
		placeholder = 'Search…'
	}: Props = $props();
	let query = $state('');
	let open = $state(false);
	let activeIdx = $state(0);

	const filtered = $derived.by(() => {
		const q = query.toLowerCase();
		if (!q) return options.slice(0, 100);
		return options
			.filter((o) => (o.label ?? o.value).toLowerCase().includes(q) || o.value.toLowerCase().includes(q))
			.slice(0, 100);
	});

	function choose(idx: number) {
		const o = filtered[idx];
		if (!o) return;
		value = o.value;
		query = o.label ?? o.value;
		open = false;
	}

	function onKey(e: KeyboardEvent) {
		if (!open) return;
		if (e.key === 'ArrowDown') {
			activeIdx = Math.min(filtered.length - 1, activeIdx + 1);
			e.preventDefault();
		} else if (e.key === 'ArrowUp') {
			activeIdx = Math.max(0, activeIdx - 1);
			e.preventDefault();
		} else if (e.key === 'Enter') {
			choose(activeIdx);
			e.preventDefault();
		} else if (e.key === 'Escape') {
			open = false;
		}
	}
</script>

<label class="relative block">
	{#if label}<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>{/if}
	<input
		type="text"
		bind:value={query}
		onfocus={() => (open = true)}
		onblur={() => setTimeout(() => (open = false), 100)}
		onkeydown={onKey}
		{placeholder}
		class="w-full rounded-md border border-[--color-border] bg-transparent px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
		role="combobox"
		aria-expanded={open}
	/>
	{#if open && filtered.length > 0}
		<ul
			role="listbox"
			class="absolute z-10 mt-1 max-h-64 w-full overflow-auto rounded-md border border-[--color-border] bg-[--color-surface-elev] text-sm"
		>
			{#each filtered as o, i (o.value)}
				<li
					role="option"
					aria-selected={i === activeIdx}
					onmousedown={() => choose(i)}
					class="cursor-pointer px-3 py-2 {i === activeIdx
						? 'bg-[--color-surface] text-[--color-text]'
						: 'text-[--color-text-muted] hover:text-[--color-text]'}"
				>
					<div>{o.label ?? o.value}</div>
					{#if o.group}<div class="mono text-[10px] text-[--color-text-muted]">{o.group}</div>{/if}
				</li>
			{/each}
		</ul>
	{/if}
</label>
```

- [ ] **Step 2: Write `Combobox.test.ts`**

```ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import Combobox from './Combobox.svelte';

describe('Combobox', () => {
	const options = [
		{ value: 'm5.large', label: 'm5.large — 2 vCPU 8 GiB' },
		{ value: 'm5.xlarge', label: 'm5.xlarge — 4 vCPU 16 GiB' },
		{ value: 'c5.large' }
	];

	it('filters options on typing', async () => {
		render(Combobox, { props: { options, value: '' } });
		const input = screen.getByRole('combobox');
		await fireEvent.focus(input);
		await fireEvent.input(input, { target: { value: 'm5' } });
		expect(screen.getAllByRole('option')).toHaveLength(2);
	});

	it('picks with keyboard', async () => {
		render(Combobox, { props: { options, value: '' } });
		const input = screen.getByRole('combobox');
		await fireEvent.focus(input);
		await fireEvent.keyDown(input, { key: 'ArrowDown' });
		await fireEvent.keyDown(input, { key: 'Enter' });
		// The listbox is closed; input value is now the second option's label.
		expect((input as HTMLInputElement).value.length).toBeGreaterThan(0);
	});
});
```

- [ ] **Step 3: Run tests**

```bash
cd web && npm run test 2>&1 | tail -10 && cd ..
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/components/Combobox.svelte web/src/lib/components/Combobox.test.ts
git commit -m "feat(web): Combobox primitive (searchable, keyboard-navigable)"
```

---

## Task 13: Shared primitive — `RegionPicker`

**Files:**
- Create: `web/src/lib/components/RegionPicker.svelte`
- Create: `web/src/lib/components/RegionPicker.test.ts`

- [ ] **Step 1: Write `RegionPicker.svelte`**

```svelte
<script lang="ts">
	import { humaniseRegion } from '$lib/utils/labels';
	import type { Region } from '$lib/api/types';

	type Props = {
		value?: string;
		regions: Region[];
		label?: string;
	};
	let { value = $bindable('us-east-1'), regions, label = 'Region' }: Props = $props();

	function continent(code: string): string {
		if (code.startsWith('us-') || code.startsWith('ca-')) return 'North America';
		if (code.startsWith('eu-')) return 'Europe';
		if (code.startsWith('ap-')) return 'Asia Pacific';
		if (code.startsWith('sa-')) return 'South America';
		if (code.startsWith('me-')) return 'Middle East';
		if (code.startsWith('af-')) return 'Africa';
		return 'Other';
	}

	const groups = $derived.by(() => {
		const buckets = new Map<string, Region[]>();
		for (const r of regions ?? []) {
			const c = continent(r.code);
			if (!buckets.has(c)) buckets.set(c, []);
			buckets.get(c)!.push(r);
		}
		return [...buckets.entries()].sort((a, b) => a[0].localeCompare(b[0]));
	});
</script>

<label class="block">
	<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>
	<select
		bind:value
		class="w-full rounded-md border border-[--color-border] bg-[--color-surface] px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
	>
		{#each groups as [continentName, rs] (continentName)}
			<optgroup label={continentName}>
				{#each rs as r (r.code)}
					<option value={r.code}>{humaniseRegion(r.code)}</option>
				{/each}
			</optgroup>
		{/each}
	</select>
</label>
```

- [ ] **Step 2: Write `RegionPicker.test.ts`**

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import RegionPicker from './RegionPicker.svelte';

describe('RegionPicker', () => {
	it('groups regions by continent', () => {
		render(RegionPicker, {
			props: {
				value: 'us-east-1',
				regions: [
					{ code: 'us-east-1' },
					{ code: 'eu-west-1' },
					{ code: 'ap-south-1' }
				]
			}
		});
		expect(screen.getByRole('combobox')).toHaveValue('us-east-1');
		// optgroup labels aren't easily role-queried; assert on innerHTML instead.
		const select = screen.getByRole('combobox');
		expect(select.innerHTML).toContain('North America');
		expect(select.innerHTML).toContain('Europe');
		expect(select.innerHTML).toContain('Asia Pacific');
	});
});
```

- [ ] **Step 3: Run tests**

```bash
cd web && npm run test 2>&1 | tail -10 && cd ..
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/components/RegionPicker.svelte web/src/lib/components/RegionPicker.test.ts
git commit -m "feat(web): RegionPicker primitive (grouped by continent)"
```

---

## Task 14: EC2 mapper (pure) with tests

**Files:**
- Create: `web/src/lib/services/ec2/mapper.ts`
- Create: `web/src/lib/services/ec2/mapper.test.ts`
- Create: `web/src/lib/services/ec2/icon.svg`

- [ ] **Step 1: Write `mapper.ts`**

```ts
import type { PricedComponent, ServiceMeta } from '../types';

export type EC2Input = {
	instanceType: string;
	operatingSystem: 'Linux' | 'Windows' | 'RHEL' | 'SUSE';
	tenancy: 'Shared' | 'Dedicated' | 'Host';
	count: number;
	hoursPerMonth: number;
};

export const meta: ServiceMeta = {
	code: 'ec2',
	label: 'EC2 (compute)',
	description: 'Virtual servers by the hour.',
	icon: '/icons/ec2.svg',
	category: 'Compute'
};

export const defaults: EC2Input = {
	instanceType: 'm5.large',
	operatingSystem: 'Linux',
	tenancy: 'Shared',
	count: 1,
	hoursPerMonth: 730
};

export function map(input: EC2Input, region: string): PricedComponent[] {
	if (input.count <= 0 || input.hoursPerMonth <= 0) return [];
	return [
		{
			service: 'AmazonEC2',
			region,
			attributes: {
				instanceType: input.instanceType,
				operatingSystem: input.operatingSystem,
				tenancy: input.tenancy,
				preInstalledSw: 'NA',
				capacitystatus: 'Used'
			},
			quantity: input.count * input.hoursPerMonth,
			unitLabel: 'hours',
			summary: `${input.count} × ${input.instanceType} in ${region}, ${input.operatingSystem}, ${input.hoursPerMonth} h/mo`
		}
	];
}
```

- [ ] **Step 2: Write `mapper.test.ts`**

```ts
import { describe, expect, it } from 'vitest';
import { defaults, map } from './mapper';

describe('EC2 mapper', () => {
	it('returns one component with resolved attributes', () => {
		const comps = map(defaults, 'us-east-1');
		expect(comps).toHaveLength(1);
		expect(comps[0].service).toBe('AmazonEC2');
		expect(comps[0].region).toBe('us-east-1');
		expect(comps[0].attributes.instanceType).toBe('m5.large');
		expect(comps[0].attributes.capacitystatus).toBe('Used');
		expect(comps[0].quantity).toBe(730);
	});

	it('multiplies quantity by count', () => {
		const [c] = map({ ...defaults, count: 3 }, 'us-east-1');
		expect(c.quantity).toBe(3 * 730);
	});

	it('returns no components for degenerate input', () => {
		expect(map({ ...defaults, count: 0 }, 'us-east-1')).toHaveLength(0);
		expect(map({ ...defaults, hoursPerMonth: 0 }, 'us-east-1')).toHaveLength(0);
	});
});
```

- [ ] **Step 3: Create a placeholder icon**

`web/src/lib/services/ec2/icon.svg`:

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="2"/><path d="M8 8h8M8 12h8M8 16h8"/></svg>
```

- [ ] **Step 4: Run tests**

```bash
cd web && npm run test 2>&1 | tail -10 && cd ..
```

Expected: 3 EC2 mapper tests PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/services/ec2/mapper.ts web/src/lib/services/ec2/mapper.test.ts web/src/lib/services/ec2/icon.svg
git commit -m "feat(web/services/ec2): mapper + defaults + tests"
```

---

## Task 15: EC2 Form component

**Files:**
- Create: `web/src/lib/services/ec2/Form.svelte`

The form composes the shared primitives and calls its mapper on every change. It doesn't hit the API — the side-sheet (built in Plan 10) owns the debounce, `/price` call, and the "Add to estimate" button. This Form just emits an `onChange` payload of `{ input, components }`.

- [ ] **Step 1: Write `Form.svelte`**

```svelte
<script lang="ts">
	import Combobox from '$lib/components/Combobox.svelte';
	import PresetSelect from '$lib/components/PresetSelect.svelte';
	import NumberInput from '$lib/components/NumberInput.svelte';
	import { defaults, map, type EC2Input } from './mapper';
	import type { PricedComponent } from '../types';

	type Props = {
		region: string;
		initial?: Partial<EC2Input>;
		onChange: (payload: { input: EC2Input; components: PricedComponent[] }) => void;
	};
	let { region, initial = {}, onChange }: Props = $props();

	let input = $state<EC2Input>({ ...defaults, ...initial });

	const instanceTypes = [
		{ value: 't3.micro', label: 't3.micro — 2 vCPU 1 GiB' },
		{ value: 't3.small', label: 't3.small — 2 vCPU 2 GiB' },
		{ value: 't3.medium', label: 't3.medium — 2 vCPU 4 GiB' },
		{ value: 't3.large', label: 't3.large — 2 vCPU 8 GiB' },
		{ value: 'm5.large', label: 'm5.large — 2 vCPU 8 GiB' },
		{ value: 'm5.xlarge', label: 'm5.xlarge — 4 vCPU 16 GiB' },
		{ value: 'm5.2xlarge', label: 'm5.2xlarge — 8 vCPU 32 GiB' },
		{ value: 'c5.large', label: 'c5.large — 2 vCPU 4 GiB' },
		{ value: 'c5.xlarge', label: 'c5.xlarge — 4 vCPU 8 GiB' },
		{ value: 'r5.large', label: 'r5.large — 2 vCPU 16 GiB' },
		{ value: 'r5.xlarge', label: 'r5.xlarge — 4 vCPU 32 GiB' }
	];
	const osOptions = [
		{ value: 'Linux', label: 'Linux' },
		{ value: 'Windows', label: 'Windows' },
		{ value: 'RHEL', label: 'RHEL' },
		{ value: 'SUSE', label: 'SUSE' }
	];
	const tenancyOptions = [
		{ value: 'Shared', label: 'Shared (default)' },
		{ value: 'Dedicated', label: 'Dedicated Instance' },
		{ value: 'Host', label: 'Dedicated Host' }
	];
	const usagePresets = [
		{ label: 'Full month (730 h)', value: 730 },
		{ label: 'Weekdays 8×5 (176 h)', value: 176 },
		{ label: 'Half month (365 h)', value: 365 }
	];

	$effect(() => {
		onChange({ input, components: map(input, region) });
	});
</script>

<div class="space-y-4">
	<Combobox
		label="Instance type"
		options={instanceTypes}
		bind:value={input.instanceType}
		placeholder="Search instance types…"
	/>
	<label class="block">
		<span class="mb-1 block text-xs text-[--color-text-muted]">Operating system</span>
		<select
			bind:value={input.operatingSystem}
			class="w-full rounded-md border border-[--color-border] bg-[--color-surface] px-3 py-2 text-sm text-[--color-text]"
		>
			{#each osOptions as o (o.value)}
				<option value={o.value}>{o.label}</option>
			{/each}
		</select>
	</label>
	<label class="block">
		<span class="mb-1 block text-xs text-[--color-text-muted]">Tenancy</span>
		<select
			bind:value={input.tenancy}
			class="w-full rounded-md border border-[--color-border] bg-[--color-surface] px-3 py-2 text-sm text-[--color-text]"
		>
			{#each tenancyOptions as o (o.value)}
				<option value={o.value}>{o.label}</option>
			{/each}
		</select>
	</label>
	<NumberInput label="How many?" unit="instances" min={1} step={1} bind:value={input.count} />
	<div>
		<PresetSelect
			label="Usage per month"
			presets={usagePresets}
			bind:value={input.hoursPerMonth}
		/>
	</div>
</div>
```

- [ ] **Step 2: Type-check**

```bash
cd web && npm run check 2>&1 | tail -5 && cd ..
```

Expected: 0 errors (warnings acceptable).

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/services/ec2/Form.svelte
git commit -m "feat(web/services/ec2): Form.svelte composes shared primitives"
```

---

## Task 16: Registry stub + wire EC2

**Files:**
- Create: `web/src/lib/services/index.ts`

- [ ] **Step 1: Write `index.ts`**

```ts
import type { ServiceEntry } from './types';
import * as ec2 from './ec2/mapper';
import EC2Form from './ec2/Form.svelte';

export const registry: Record<string, ServiceEntry<any>> = {
	ec2: {
		meta: ec2.meta,
		defaults: ec2.defaults,
		map: ec2.map,
		Form: EC2Form
	}
};

export type ServiceKey = keyof typeof registry;

export function listServices(): ServiceEntry<any>[] {
	return Object.values(registry);
}
```

- [ ] **Step 2: Type-check + build**

```bash
cd web && npm run check 2>&1 | tail -5 && npm run build 2>&1 | tail -5 && cd ..
```

Expected: 0 type errors, build succeeds.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/services/index.ts
git commit -m "feat(web): service registry with EC2 pilot entry"
```

---

## Task 17: Frontend full sweep + push

- [ ] **Step 1: Run tests + build**

```bash
cd web && npm run test 2>&1 | tail -6 && npm run check 2>&1 | tail -5 && cd ..
```

Expected: all tests PASS, 0 type errors.

- [ ] **Step 2: Confirm compose still builds and starts**

```bash
docker compose build web 2>&1 | tail -2
docker compose up -d web 2>&1 | tail -2
sleep 5
curl -sf http://localhost:3000/ | grep -q 'AWS Calculator' && echo "web OK"
```

- [ ] **Step 3: Push**

```bash
git push origin main
```

---

## Self-review notes

- **Spec coverage.**
  - Migration 0006 → Task 1.
  - `estimate_items` new columns round-trip → Tasks 2, 3, 4, 7.
  - Reprice sums components → Task 5.
  - Export renders curated summary → Task 6.
  - `services/` package with `Form + mapper + registry` → Tasks 9, 14, 15, 16.
  - Shared primitives (`Combobox`, `PresetSelect`, `NumberInput`, `RegionPicker`) → Tasks 10, 11, 12, 13.
  - EC2 pilot mapper + form + tests → Tasks 14, 15.
  - vitest wiring → Task 9.
- **Placeholder scan.** No `TBD`/`TODO`/`similar to task N`. Test bodies contain real assertions. All SQL, Go, TS, and Svelte code blocks are complete.
- **Type consistency.** `Component` (Go, `estimates.Component`) and `PricedComponent` (TS API, and TS mapper) intentionally use different types — Go persists resolved prices, TS mapper returns un-priced components that the side-sheet will resolve via `/price`. `ServiceEntry.Form` uses Svelte's `Component` type from `svelte`. The registry key `'ec2'` is used consistently as the `service_key` on both frontend and backend.
- **Deferred to Plan 10.** The side-sheet, `/estimates/[id]` editor, landing page, `/services?to=&edit=` context, and the deletion of the cart store all live in Plan 10. Plan 09 leaves the current UI intact: EC2 curated forms exist in code but aren't yet reachable from any user-visible route.
