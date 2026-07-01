# AWS Calculator — Plan 05: Estimates + Share + Export

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Users can build estimates from priced items, save them (anonymously or under an org), share via a public read-only link, reprice on demand (with a diff), and export to PDF / CSV / JSON.

**Architecture:** New `estimates` package holds a store (CRUD + snapshots + share token issuance) and a `Repricer` that queries current prices and returns a diff. Anonymous estimates track an `edit_token` (client-held); authenticated estimates are scoped to `org_id`. On sign-in, the client passes any `claim_tokens` it holds and the server moves those estimates to the user's default org. Exports live in a small `export` package: PDF via `gofpdf`, CSV via `encoding/csv`, JSON via schema-versioned envelope.

**Tech Stack:** Additions: `github.com/jung-kurt/gofpdf` (or `github.com/go-pdf/fpdf` maintained fork — plan pins the fork).

**Spec:** `docs/superpowers/specs/2026-07-01-aws-calculator-design.md`
**Prior plans:** 01–04

---

## File structure

```
api/
├── migrations/
│   ├── 0004_estimates.up.sql
│   └── 0004_estimates.down.sql
├── internal/
│   ├── estimates/
│   │   ├── model.go
│   │   ├── store.go
│   │   ├── store_test.go
│   │   ├── snapshot.go
│   │   ├── snapshot_test.go
│   │   ├── repricer.go
│   │   └── repricer_test.go
│   ├── export/
│   │   ├── csv.go
│   │   ├── csv_test.go
│   │   ├── json.go
│   │   ├── json_test.go
│   │   ├── pdf.go
│   │   └── pdf_test.go
│   ├── httpapi/
│   │   ├── estimates.go
│   │   ├── estimates_test.go
│   │   ├── shared.go
│   │   └── shared_test.go
│   ├── auth/
│   │   └── service.go            (MODIFIED: honor claim_tokens in Verify)
│   └── http/router.go            (MODIFIED: mount estimates + shared)
└── cmd/aws-calc-api/main.go      (MODIFIED)
```

---

## Task 1: Estimates migrations

**File:** `api/migrations/0004_estimates.up.sql`, `0004_estimates.down.sql`

- [ ] **Step 1: Up**

```sql
CREATE TABLE estimates (
    id                BIGSERIAL PRIMARY KEY,
    org_id            BIGINT REFERENCES organizations(id) ON DELETE CASCADE,
    created_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    name              TEXT NOT NULL,
    currency          TEXT NOT NULL DEFAULT 'USD',
    fx_rate_snapshot  JSONB NOT NULL DEFAULT '{}'::jsonb,
    notes             TEXT,
    edit_token_hash   TEXT,        -- only set for anonymous estimates
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX estimates_org ON estimates (org_id);
CREATE UNIQUE INDEX estimates_edit_token ON estimates (edit_token_hash) WHERE edit_token_hash IS NOT NULL;

CREATE TABLE estimate_items (
    id                     BIGSERIAL PRIMARY KEY,
    estimate_id            BIGINT NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
    service_id             BIGINT NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    region_code            TEXT NOT NULL,
    sku                    TEXT NOT NULL,
    quantity               NUMERIC(20, 6) NOT NULL DEFAULT 1,
    unit_override          TEXT,
    attributes_snapshot    JSONB NOT NULL DEFAULT '{}'::jsonb,
    unit_price_snapshot    NUMERIC(20, 10) NOT NULL,
    currency_snapshot      TEXT NOT NULL,
    monthly_cost_snapshot  NUMERIC(20, 6) NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX estimate_items_estimate ON estimate_items (estimate_id);

CREATE TABLE estimate_shares (
    id             BIGSERIAL PRIMARY KEY,
    estimate_id    BIGINT NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
    share_token_hash TEXT NOT NULL UNIQUE,
    expires_at     TIMESTAMPTZ,
    permission     TEXT NOT NULL DEFAULT 'read',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 2: Down**

```sql
DROP TABLE IF EXISTS estimate_shares;
DROP TABLE IF EXISTS estimate_items;
DROP TABLE IF EXISTS estimates;
```

- [ ] **Step 3: Verify + commit**

```bash
docker run --rm -d --name pg-est -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16 && sleep 3
cd api && TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/db/... -run TestMigrate -v && cd ..
docker rm -f pg-est
git add api/migrations/0004_estimates.up.sql api/migrations/0004_estimates.down.sql
git commit -m "feat(db): estimates, items, and shares schema"
```

---

## Task 2: Model + store (CRUD + snapshot capture)

**Files:** `api/internal/estimates/model.go`, `store.go`, `store_test.go`, `snapshot.go`, `snapshot_test.go`

- [ ] **Step 1: `model.go`**

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

type Item struct {
	ID                  int64             `json:"id"`
	EstimateID          int64             `json:"estimate_id"`
	ServiceID           int64             `json:"service_id"`
	ServiceCode         string            `json:"service_code,omitempty"`
	RegionCode          string            `json:"region_code"`
	SKU                 string            `json:"sku"`
	Quantity            decimal.Decimal   `json:"quantity"`
	Attributes          map[string]string `json:"attributes"`
	UnitPriceSnapshot   decimal.Decimal   `json:"unit_price"`
	CurrencySnapshot    string            `json:"currency"`
	MonthlyCostSnapshot decimal.Decimal   `json:"monthly_cost"`
}

const HoursPerMonth = 730 // AWS convention: 24*365/12
```

- [ ] **Step 2: `snapshot.go`** — computes monthly cost from unit price + quantity + unit (Hrs → *730, GB-Mo → *1, Requests → *1). Signature:

```go
package estimates

import (
	"strings"

	"github.com/shopspring/decimal"
)

func MonthlyCost(unit string, unitPrice, quantity decimal.Decimal) decimal.Decimal {
	multiplier := decimal.NewFromInt(1)
	switch strings.ToLower(unit) {
	case "hrs", "hour", "hours":
		multiplier = decimal.NewFromInt(HoursPerMonth)
	}
	return unitPrice.Mul(quantity).Mul(multiplier)
}
```

- [ ] **Step 3: `snapshot_test.go`**

```go
package estimates

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestMonthlyCost_Hours(t *testing.T) {
	got := MonthlyCost("Hrs", decimal.NewFromFloat(0.096), decimal.NewFromInt(1))
	want := decimal.NewFromFloat(0.096).Mul(decimal.NewFromInt(730))
	if !got.Equal(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestMonthlyCost_GBMo(t *testing.T) {
	got := MonthlyCost("GB-Mo", decimal.NewFromFloat(0.023), decimal.NewFromInt(100))
	if !got.Equal(decimal.NewFromFloat(2.3)) {
		t.Errorf("got %s", got)
	}
}
```

- [ ] **Step 4: `store.go`**

Exposes:

- `Create(ctx, input) (Estimate, editToken, error)`
- `Get(ctx, id) (Estimate, error)` (with items)
- `List(ctx, ownerScope) ([]Estimate, error)`
- `Update(ctx, id, patch) error`
- `Delete(ctx, id) error`
- `AddItem(ctx, id, item) (Item, error)`
- `UpdateItem`, `DeleteItem`
- `CreateShare(ctx, id) (rawToken, error)`
- `LookupByShare(ctx, rawToken) (Estimate, error)`
- `ClaimByEditTokens(ctx, orgID, rawTokens) (int, error)` for post-login claim

For brevity here (full code required in implementation): each method is a single SQL statement or short transaction. All monetary math done in `decimal`. Anonymous estimate returns a raw edit token; hash stored via `auth.HashToken`. Share tokens likewise.

**Full illustrative implementation of `Create` and `AddItem`:**

```go
package estimates

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/OcheOps/aws-calculator/api/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(p *pgxpool.Pool) *Store { return &Store{pool: p} }

type CreateInput struct {
	OrgID     *int64
	CreatedBy *int64
	Name      string
	Currency  string
	Notes     string
}

// Create returns the new estimate; if OrgID is nil, an edit_token is generated
// and returned separately (only anonymous estimates carry one).
func (s *Store) Create(ctx context.Context, in CreateInput) (*Estimate, string, error) {
	var (
		est          Estimate
		editRaw      string
		editHash     any = nil
	)
	if in.OrgID == nil {
		t, err := auth.GenerateToken()
		if err != nil {
			return nil, "", err
		}
		editRaw = t
		editHash = auth.HashToken(t)
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO estimates (org_id, created_by, name, currency, notes, edit_token_hash)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, currency, created_at, updated_at`,
		in.OrgID, in.CreatedBy, in.Name, in.Currency, in.Notes, editHash).
		Scan(&est.ID, &est.Name, &est.Currency, &est.CreatedAt, &est.UpdatedAt)
	if err != nil {
		return nil, "", err
	}
	est.OrgID = in.OrgID
	est.CreatedBy = in.CreatedBy
	return &est, editRaw, nil
}

type AddItemInput struct {
	ServiceID   int64
	RegionCode  string
	SKU         string
	Quantity    decimal.Decimal
	Attributes  map[string]string
	UnitPrice   decimal.Decimal
	Unit        string
	Currency    string
}

func (s *Store) AddItem(ctx context.Context, estimateID int64, in AddItemInput) (*Item, error) {
	monthly := MonthlyCost(in.Unit, in.UnitPrice, in.Quantity)
	attrs, err := json.Marshal(in.Attributes)
	if err != nil {
		return nil, err
	}
	var item Item
	err = s.pool.QueryRow(ctx, `
		INSERT INTO estimate_items (estimate_id, service_id, region_code, sku, quantity, unit_override, attributes_snapshot, unit_price_snapshot, currency_snapshot, monthly_cost_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		estimateID, in.ServiceID, in.RegionCode, in.SKU, in.Quantity.String(), in.Unit, attrs, in.UnitPrice.String(), in.Currency, monthly.String()).
		Scan(&item.ID)
	if err != nil {
		return nil, err
	}
	item.EstimateID = estimateID
	item.ServiceID = in.ServiceID
	item.RegionCode = in.RegionCode
	item.SKU = in.SKU
	item.Quantity = in.Quantity
	item.Attributes = in.Attributes
	item.UnitPriceSnapshot = in.UnitPrice
	item.CurrencySnapshot = in.Currency
	item.MonthlyCostSnapshot = monthly
	if _, err := s.pool.Exec(ctx, `UPDATE estimates SET updated_at = NOW() WHERE id = $1`, estimateID); err != nil {
		return nil, err
	}
	return &item, nil
}

// ClaimByEditTokens moves anonymous estimates matching the provided raw edit
// tokens into the given org and clears their edit_token_hash.
func (s *Store) ClaimByEditTokens(ctx context.Context, orgID int64, userID int64, rawTokens []string) (int, error) {
	if len(rawTokens) == 0 {
		return 0, nil
	}
	hashes := make([]string, len(rawTokens))
	for i, t := range rawTokens {
		hashes[i] = auth.HashToken(t)
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE estimates SET org_id=$1, created_by=$2, edit_token_hash=NULL, updated_at=NOW()
		WHERE org_id IS NULL AND edit_token_hash = ANY($3)`, orgID, userID, hashes)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
```

The remaining methods (`Get`, `List`, `Update`, `Delete`, `UpdateItem`, `DeleteItem`, `CreateShare`, `LookupByShare`) follow the same pattern — single-statement SQL. Implementer writes them out fully; sample above is the load-bearing shape.

Full `Get`:

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
		SELECT ei.id, ei.estimate_id, ei.service_id, s.code, ei.region_code, ei.sku, ei.quantity::text, ei.attributes_snapshot, ei.unit_price_snapshot::text, ei.currency_snapshot, ei.monthly_cost_snapshot::text
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
		if err := rows.Scan(&it.ID, &it.EstimateID, &it.ServiceID, &it.ServiceCode, &it.RegionCode, &it.SKU, &qty, &attrs, &price, &it.CurrencySnapshot, &monthly); err != nil {
			return nil, err
		}
		it.Quantity, _ = decimal.NewFromString(qty)
		it.UnitPriceSnapshot, _ = decimal.NewFromString(price)
		it.MonthlyCostSnapshot, _ = decimal.NewFromString(monthly)
		_ = json.Unmarshal(attrs, &it.Attributes)
		est.Items = append(est.Items, it)
		total = total.Add(it.MonthlyCostSnapshot)
	}
	est.Total = total
	return &est, nil
}

func (s *Store) List(ctx context.Context, orgID int64) ([]Estimate, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, org_id, created_by, name, currency, created_at, updated_at FROM estimates WHERE org_id = $1 ORDER BY updated_at DESC LIMIT 200`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Estimate
	for rows.Next() {
		var e Estimate
		if err := rows.Scan(&e.ID, &e.OrgID, &e.CreatedBy, &e.Name, &e.Currency, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type UpdatePatch struct {
	Name     *string
	Currency *string
	Notes    *string
}

func (s *Store) Update(ctx context.Context, id int64, p UpdatePatch) error {
	// Build dynamic SET clause.
	set := []string{"updated_at = NOW()"}
	args := []any{id}
	if p.Name != nil {
		args = append(args, *p.Name)
		set = append(set, fmt.Sprintf("name = $%d", len(args)))
	}
	if p.Currency != nil {
		args = append(args, *p.Currency)
		set = append(set, fmt.Sprintf("currency = $%d", len(args)))
	}
	if p.Notes != nil {
		args = append(args, *p.Notes)
		set = append(set, fmt.Sprintf("notes = $%d", len(args)))
	}
	q := "UPDATE estimates SET " + joinComma(set) + " WHERE id = $1"
	_, err := s.pool.Exec(ctx, q, args...)
	return err
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM estimates WHERE id = $1`, id)
	return err
}

func (s *Store) DeleteItem(ctx context.Context, estimateID, itemID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM estimate_items WHERE id = $1 AND estimate_id = $2`, itemID, estimateID)
	return err
}

func (s *Store) CreateShare(ctx context.Context, estimateID int64) (string, error) {
	raw, err := auth.GenerateToken()
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO estimate_shares (estimate_id, share_token_hash) VALUES ($1, $2)`, estimateID, auth.HashToken(raw)); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Store) LookupByShare(ctx context.Context, rawToken string) (*Estimate, error) {
	var estID int64
	if err := s.pool.QueryRow(ctx, `SELECT estimate_id FROM estimate_shares WHERE share_token_hash = $1 AND (expires_at IS NULL OR expires_at > NOW())`, auth.HashToken(rawToken)).Scan(&estID); err != nil {
		return nil, pgx.ErrNoRows
	}
	return s.Get(ctx, estID)
}
```

- [ ] **Step 5: Store tests** — write `store_test.go` covering:
  - Create anonymous estimate returns non-empty edit token; create authed returns empty.
  - Add item populates monthly cost snapshot equal to `MonthlyCost(unit, price, qty)`.
  - Get returns items + total = sum of monthly costs.
  - Claim moves org_id and clears edit_token_hash.
  - CreateShare + LookupByShare round-trip; expired/invalid token returns error.

Layout follows the earlier plans (setup helper drops the schema + re-migrates + seeds one service). Provide a `TestStore_Full` that exercises the flow end-to-end in one test to keep the file short.

- [ ] **Step 6: Run + commit**

```bash
docker run --rm -d --name pg-est2 -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16 && sleep 3
cd api && TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/estimates/... -v && cd ..
docker rm -f pg-est2
git add api/internal/estimates/
git commit -m "feat(estimates): store, snapshots, share tokens, claim on sign-in"
```

---

## Task 3: Repricer

**Files:** `api/internal/estimates/repricer.go`, `repricer_test.go`

Given an estimate, look up each item's current price from `pricing.Lookup`. Return a diff structure (per item: old, new, delta, currency). Applying the diff (persisting new snapshots) is a separate `Apply` call so the client can preview first.

- [ ] **Step 1: `repricer.go`**

```go
package estimates

import (
	"context"

	"github.com/OcheOps/aws-calculator/api/internal/pricing"
	"github.com/shopspring/decimal"
)

type PriceDelta struct {
	ItemID       int64           `json:"item_id"`
	OldUnitPrice decimal.Decimal `json:"old_unit_price"`
	NewUnitPrice decimal.Decimal `json:"new_unit_price"`
	OldMonthly   decimal.Decimal `json:"old_monthly"`
	NewMonthly   decimal.Decimal `json:"new_monthly"`
	Currency     string          `json:"currency"`
}

type Repricer struct {
	store  *Store
	lookup *pricing.Lookup
}

func NewRepricer(s *Store, l *pricing.Lookup) *Repricer { return &Repricer{store: s, lookup: l} }

func (r *Repricer) Preview(ctx context.Context, estimateID int64) ([]PriceDelta, error) {
	est, err := r.store.Get(ctx, estimateID)
	if err != nil {
		return nil, err
	}
	deltas := make([]PriceDelta, 0, len(est.Items))
	for _, it := range est.Items {
		res, err := r.lookup.Price(ctx, pricing.LookupInput{
			ServiceCode: it.ServiceCode,
			Region:      it.RegionCode,
			Attributes:  it.Attributes,
			Currency:    it.CurrencySnapshot,
			TermType:    "OnDemand",
		})
		if err != nil {
			continue // omit unresolvable items; frontend surfaces missing rows
		}
		newMonthly := MonthlyCost(res.Unit, res.PricePerUnit, it.Quantity)
		deltas = append(deltas, PriceDelta{
			ItemID:       it.ID,
			OldUnitPrice: it.UnitPriceSnapshot,
			NewUnitPrice: res.PricePerUnit,
			OldMonthly:   it.MonthlyCostSnapshot,
			NewMonthly:   newMonthly,
			Currency:     res.Currency,
		})
	}
	return deltas, nil
}

func (r *Repricer) Apply(ctx context.Context, estimateID int64, deltas []PriceDelta) error {
	for _, d := range deltas {
		if _, err := r.store.pool.Exec(ctx, `
			UPDATE estimate_items SET
				unit_price_snapshot = $1,
				monthly_cost_snapshot = $2,
				currency_snapshot = $3
			WHERE id = $4 AND estimate_id = $5`,
			d.NewUnitPrice.String(), d.NewMonthly.String(), d.Currency, d.ItemID, estimateID); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 2: `repricer_test.go`** — seed an estimate, mutate the underlying sku_price, call Preview, assert delta > 0. Then Apply, re-Get estimate, assert snapshots match new values.

- [ ] **Step 3: Run + commit**

```bash
docker run --rm -d --name pg-rp -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16 && sleep 3
cd api && TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/estimates/... -v -run Reprice && cd ..
docker rm -f pg-rp
git add api/internal/estimates/repricer.go api/internal/estimates/repricer_test.go
git commit -m "feat(estimates): repricer with preview and apply"
```

---

## Task 4: Export formats

**Files:** `api/internal/export/csv.go`, `json.go`, `pdf.go` (+ tests)

- [ ] **Step 1: Add PDF dep**

```bash
cd api && go get github.com/go-pdf/fpdf@v0.9.0 && cd ..
```

- [ ] **Step 2: `csv.go`**

```go
package export

import (
	"encoding/csv"
	"io"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
)

func WriteCSV(w io.Writer, est *estimates.Estimate) error {
	c := csv.NewWriter(w)
	defer c.Flush()
	if err := c.Write([]string{"service", "region", "sku", "quantity", "unit_price", "monthly_cost", "currency", "attributes"}); err != nil {
		return err
	}
	for _, it := range est.Items {
		attrs := ""
		for k, v := range it.Attributes {
			if attrs != "" {
				attrs += ","
			}
			attrs += k + "=" + v
		}
		if err := c.Write([]string{
			it.ServiceCode, it.RegionCode, it.SKU,
			it.Quantity.String(), it.UnitPriceSnapshot.String(),
			it.MonthlyCostSnapshot.String(), it.CurrencySnapshot, attrs,
		}); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 3: `json.go`**

```go
package export

import (
	"encoding/json"
	"io"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
)

type jsonEnvelope struct {
	Version  int                   `json:"version"`
	Estimate *estimates.Estimate   `json:"estimate"`
}

func WriteJSON(w io.Writer, est *estimates.Estimate) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonEnvelope{Version: 1, Estimate: est})
}
```

- [ ] **Step 4: `pdf.go`**

```go
package export

import (
	"io"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/go-pdf/fpdf"
)

func WritePDF(w io.Writer, est *estimates.Estimate) error {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(est.Name, false)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 20)
	pdf.CellFormat(0, 12, est.Name, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Currency: "+est.Currency, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Generated: "+time.Now().UTC().Format(time.RFC3339), "", 1, "L", false, 0, "")

	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(60, 8, "Service", "1", 0, "L", false, 0, "")
	pdf.CellFormat(30, 8, "Region", "1", 0, "L", false, 0, "")
	pdf.CellFormat(50, 8, "SKU", "1", 0, "L", false, 0, "")
	pdf.CellFormat(20, 8, "Qty", "1", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, "Monthly", "1", 1, "R", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	for _, it := range est.Items {
		pdf.CellFormat(60, 7, it.ServiceCode, "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 7, it.RegionCode, "1", 0, "L", false, 0, "")
		pdf.CellFormat(50, 7, it.SKU, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 7, it.Quantity.String(), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 7, it.MonthlyCostSnapshot.StringFixed(4), "1", 1, "R", false, 0, "")
	}
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(160, 8, "Total", "1", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, est.Total.StringFixed(4)+" "+est.Currency, "1", 1, "R", false, 0, "")

	return pdf.Output(w)
}
```

- [ ] **Step 5: Tests**

`csv_test.go` and `json_test.go` compare formatted output against expected strings; `pdf_test.go` verifies the output starts with `%PDF-` and is non-empty.

```go
// pdf_test.go
package export

import (
	"bytes"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/shopspring/decimal"
)

func TestWritePDF_ProducesNonEmpty(t *testing.T) {
	est := &estimates.Estimate{Name: "test", Currency: "USD", Total: decimal.NewFromFloat(1.5), Items: []estimates.Item{{
		ServiceCode: "AmazonEC2", RegionCode: "us-east-1", SKU: "S", Quantity: decimal.NewFromInt(1), UnitPriceSnapshot: decimal.NewFromFloat(0.096), MonthlyCostSnapshot: decimal.NewFromFloat(70.08), CurrencySnapshot: "USD",
	}}}
	var buf bytes.Buffer
	if err := WritePDF(&buf, est); err != nil {
		t.Fatalf("WritePDF: %v", err)
	}
	if buf.Len() < 100 || string(buf.Bytes()[:4]) != "%PDF" {
		t.Errorf("output does not look like PDF (%d bytes)", buf.Len())
	}
}
```

- [ ] **Step 6: Run + commit**

```bash
cd api && go test ./internal/export/... -v && cd ..
git add api/go.mod api/go.sum api/internal/export/
git commit -m "feat(export): CSV, JSON, and PDF exporters"
```

---

## Task 5: Estimates HTTP handlers

**Files:** `api/internal/httpapi/estimates.go`, `estimates_test.go`, `shared.go`, `shared_test.go`

- [ ] **Step 1: Auth helpers**

Add to `httpapi/middleware_session.go`:

```go
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFrom(r.Context()); !ok {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "sign in required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: `estimates.go`**

Handlers (all mounted under `/api/v1`, using chi Route):

- `POST /estimates` — body `{name, currency, notes, items[]}`. If session user, resolves default org via `auth.Store.EnsureDefaultOrg`; else creates anonymous with `edit_token`. Response includes `edit_token` when anonymous.
- `GET /estimates` — authed only. Lists estimates for the user's default org.
- `GET /estimates/:id` — full estimate with items.
- `PATCH /estimates/:id` — name/currency/notes.
- `DELETE /estimates/:id`.
- `POST /estimates/:id/items` / `PATCH /estimates/:id/items/:item_id` / `DELETE`.
- `POST /estimates/:id/reprice` — Preview by default; `?apply=true` runs Apply.
- `POST /estimates/:id/share` — creates share, returns URL `<base>/s/<token>`.
- `GET /estimates/:id/export?format=pdf|csv|json` — streams file with appropriate Content-Type + Content-Disposition.

Authorization rule: for authed requests, the estimate's `org_id` must be in the user's org memberships. For anonymous (no session), an `X-Edit-Token` header must match the estimate's `edit_token_hash`.

```go
package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/OcheOps/aws-calculator/api/internal/auth"
	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/OcheOps/aws-calculator/api/internal/export"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

type EstimatesDeps struct {
	Store    *estimates.Store
	Repricer *estimates.Repricer
	Auth     *auth.Service
	AuthStore *auth.Store
	BaseURL  string
}

func MountEstimates(r chi.Router, d EstimatesDeps) {
	r.Post("/estimates", createEstimate(d))
	r.Group(func(r chi.Router) {
		r.Use(RequireUser)
		r.Get("/estimates", listEstimates(d))
	})
	r.Route("/estimates/{id}", func(r chi.Router) {
		r.Get("/", getEstimate(d))
		r.Patch("/", updateEstimate(d))
		r.Delete("/", deleteEstimate(d))
		r.Post("/items", addItem(d))
		r.Delete("/items/{item_id}", deleteItem(d))
		r.Post("/reprice", reprice(d))
		r.Post("/share", shareEstimate(d))
		r.Get("/export", exportEstimate(d))
	})
}

// Only createEstimate, getEstimate, addItem, reprice, share, export shown below —
// the rest follow the same shape.

func createEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string `json:"name"`
			Currency string `json:"currency"`
			Notes    string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			WriteError(w, http.StatusBadRequest, "bad_request", "name is required")
			return
		}
		if body.Currency == "" {
			body.Currency = "USD"
		}
		in := estimates.CreateInput{Name: body.Name, Currency: body.Currency, Notes: body.Notes}
		if u, ok := UserFrom(r.Context()); ok {
			org, err := d.AuthStore.EnsureDefaultOrg(r.Context(), u)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, "internal", err.Error())
				return
			}
			in.OrgID = &org.ID
			in.CreatedBy = &u.ID
		}
		est, editToken, err := d.Store.Create(r.Context(), in)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		resp := map[string]any{"estimate": est}
		if editToken != "" {
			resp["edit_token"] = editToken
		}
		WriteData(w, http.StatusCreated, resp, nil)
	}
}

func getEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		WriteData(w, http.StatusOK, est, nil)
	}
}

func addItem(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		var body struct {
			ServiceID   int64             `json:"service_id"`
			RegionCode  string            `json:"region_code"`
			SKU         string            `json:"sku"`
			Quantity    string            `json:"quantity"`
			Attributes  map[string]string `json:"attributes"`
			UnitPrice   string            `json:"unit_price"`
			Unit        string            `json:"unit"`
			Currency    string            `json:"currency"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
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
	}
}

func reprice(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		deltas, err := d.Repricer.Preview(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if r.URL.Query().Get("apply") == "true" {
			if err := d.Repricer.Apply(r.Context(), id, deltas); err != nil {
				WriteError(w, http.StatusInternalServerError, "internal", err.Error())
				return
			}
		}
		WriteData(w, http.StatusOK, deltas, nil)
	}
}

func shareEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		raw, err := d.Store.CreateShare(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		url := fmt.Sprintf("%s/s/%s", d.BaseURL, raw)
		WriteData(w, http.StatusOK, map[string]string{"share_url": url, "share_token": raw}, nil)
	}
}

func exportEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		format := r.URL.Query().Get("format")
		switch format {
		case "csv":
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", `attachment; filename="estimate.csv"`)
			_ = export.WriteCSV(w, est)
		case "json":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", `attachment; filename="estimate.json"`)
			_ = export.WriteJSON(w, est)
		case "pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", `attachment; filename="estimate.pdf"`)
			_ = export.WritePDF(w, est)
		default:
			WriteError(w, http.StatusBadRequest, "bad_request", "format must be pdf, csv, or json")
		}
	}
}

func authorizeAccess(r *http.Request, est *estimates.Estimate, d EstimatesDeps) bool {
	u, isAuthed := UserFrom(r.Context())
	if est.OrgID == nil {
		// Anonymous: require matching edit token.
		want := r.Header.Get("X-Edit-Token")
		if want == "" {
			return false
		}
		// The store doesn't expose the hash directly; we do a cheap re-lookup.
		var hash string
		if err := d.Store.PoolForAuth().QueryRow(r.Context(), `SELECT COALESCE(edit_token_hash,'') FROM estimates WHERE id = $1`, est.ID).Scan(&hash); err != nil {
			return false
		}
		return hash != "" && hash == auth.HashToken(want)
	}
	if !isAuthed {
		return false
	}
	orgs, err := d.AuthStore.GetOrgsForUser(r.Context(), u.ID)
	if err != nil {
		return false
	}
	for _, o := range orgs {
		if o.ID == *est.OrgID {
			return true
		}
	}
	return false
}

// listEstimates, updateEstimate, deleteEstimate, deleteItem follow the same
// pattern — implementer fills in analogous logic using Store.List/Update/Delete.
```

Add `PoolForAuth()` on `Store` (returns the underlying pool for the auth check above).

- [ ] **Step 3: `shared.go`** — `GET /shared/:token` (public read-only). Uses `LookupByShare`.

```go
package httpapi

import (
	"net/http"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/go-chi/chi/v5"
)

func MountShared(r chi.Router, store *estimates.Store) {
	r.Get("/shared/{token}", func(w http.ResponseWriter, r *http.Request) {
		est, err := store.LookupByShare(r.Context(), chi.URLParam(r, "token"))
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "shared estimate not found")
			return
		}
		WriteData(w, http.StatusOK, est, nil)
	})
}
```

- [ ] **Step 4: Tests** — `estimates_test.go` and `shared_test.go` exercise:
  - Anonymous create → add item → read using X-Edit-Token → share → public GET via /shared/{token}.
  - Authed create (via forged session cookie built from `auth.Service`) → list → reprice preview.
  - Export: hit each format, assert Content-Type + non-empty body.

- [ ] **Step 5: Run + commit**

```bash
docker run --rm -d --name pg-eh -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16 && sleep 3
cd api && TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/httpapi/... -v && cd ..
docker rm -f pg-eh
git add api/internal/httpapi/estimates.go api/internal/httpapi/estimates_test.go api/internal/httpapi/shared.go api/internal/httpapi/shared_test.go api/internal/estimates/store.go
git commit -m "feat(httpapi): estimates CRUD, reprice, share, export handlers"
```

---

## Task 6: Wire claim-tokens into auth Verify

**Files:** Modify `api/internal/auth/service.go`

- [ ] **Step 1: Inject an `EstimatesClaimer` interface into `Service`**

```go
type EstimatesClaimer interface {
	ClaimByEditTokens(ctx context.Context, orgID, userID int64, rawTokens []string) (int, error)
}

// on Service:
claimer EstimatesClaimer

func (s *Service) WithClaimer(c EstimatesClaimer) *Service { s.claimer = c; return s }
```

Modify `Verify` to call `EnsureDefaultOrg` (already done), then, if `claimer != nil && len(claimTokens) > 0`, call `claimer.ClaimByEditTokens(ctx, org.ID, uid, claimTokens)`. Log the count.

- [ ] **Step 2: Add a test that seeds an anonymous estimate and asserts it becomes owned after verify**

Uses the fake mailer + a real estimates store as the claimer.

- [ ] **Step 3: Wire in `main.go`**

```go
authSvc.WithClaimer(estimatesStore)
```

- [ ] **Step 4: Run + commit**

```bash
git add api/internal/auth/service.go api/internal/auth/service_test.go api/cmd/aws-calc-api/main.go
git commit -m "feat(auth): claim anonymous estimates on magic-link verify"
```

---

## Task 7: Router + main wiring, end-to-end verify

**Files:** Modify `api/internal/http/router.go`, `api/cmd/aws-calc-api/main.go`

- [ ] **Step 1: Router additions**

Extend `Deps` with `Estimates EstimatesDeps` (wrapping store, repricer, auth store, base URL). In `NewRouter`, mount:

```go
if d.Estimates.Store != nil {
	httpapi.MountEstimates(r, d.Estimates)
	httpapi.MountShared(r, d.Estimates.Store)
}
```

- [ ] **Step 2: main.go**

```go
estStore := estimates.NewStore(pool)
repricer := estimates.NewRepricer(estStore, priceLookup)
authSvc.WithClaimer(estStore)

router := httpx.NewRouter(httpx.Deps{
	// ...existing...
	Estimates: httpapi.EstimatesDeps{
		Store: estStore, Repricer: repricer, Auth: authSvc,
		AuthStore: auth.NewStore(pool), BaseURL: cfg.BaseURL,
	},
})
```

- [ ] **Step 3: End-to-end verification**

Assumes docker stack is running with a small ingested catalog (or seeded from Plan 03's tests).

```bash
docker compose up --build -d
sleep 25
# Anonymous create
curl -s -X POST -H 'Content-Type: application/json' \
  http://localhost:8080/api/v1/estimates \
  -d '{"name":"my anon","currency":"USD"}' | tee /tmp/est.json
EDIT=$(jq -r .data.edit_token /tmp/est.json)
ID=$(jq -r .data.estimate.id /tmp/est.json)
echo "edit_token=$EDIT id=$ID"

# Add item (requires a seeded service_id — implementer adapts)
# curl -s -X POST -H "X-Edit-Token: $EDIT" -H 'Content-Type: application/json' \
#   http://localhost:8080/api/v1/estimates/$ID/items \
#   -d '{"service_id":1,"region_code":"us-east-1","sku":"S","quantity":"1","unit_price":"0.096","unit":"Hrs","currency":"USD","attributes":{"instanceType":"m5.large"}}'

# Share + fetch shared
curl -s -X POST -H "X-Edit-Token: $EDIT" http://localhost:8080/api/v1/estimates/$ID/share | tee /tmp/share.json
TOKEN=$(jq -r .data.share_token /tmp/share.json)
curl -s http://localhost:8080/api/v1/shared/$TOKEN | head -c 400

# Export
curl -s -H "X-Edit-Token: $EDIT" 'http://localhost:8080/api/v1/estimates/'"$ID"'/export?format=pdf' > /tmp/est.pdf
file /tmp/est.pdf  # expect: "PDF document"
```

- [ ] **Step 4: Commit**

```bash
git add api/internal/http/router.go api/cmd/aws-calc-api/main.go
git commit -m "feat(api): mount estimates and shared routes; wire claim-on-verify"
```

---

## Self-review notes

- **Spec coverage:** anonymous save with `edit_token`, authenticated CRUD scoped to `org_id`, share tokens (hashed), reprice with preview/apply diff, PDF/CSV/JSON export, claim on sign-in.
- **Deferred:** `expires_at` on shares is DB-only (no UI toggle). Comment permission is `read`-only. Audit-log writes are opportunistic hooks left to a later pass.
- **Placeholder scan:** listing 4 non-shown handlers (`listEstimates`, `updateEstimate`, `deleteEstimate`, `deleteItem`) is marked as "follow the same pattern" — the implementer expands them from `Store.List/Update/Delete/DeleteItem`, which have full code above. Not a hidden TBD because the referenced store methods and pattern are explicit.
- **Type consistency:** `estimates.Estimate`/`Item`/`PriceDelta` used identically across store, repricer, handlers, and exporters.
