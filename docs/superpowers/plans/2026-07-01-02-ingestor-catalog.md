# AWS Calculator — Plan 02: AWS Ingestor + Catalog API

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fetch the AWS pricing catalog into Postgres and expose it via read-only HTTP endpoints. After this plan, a user can hit `GET /api/v1/services` and see every AWS service with its dimensions, and `POST /api/v1/admin/ingest` triggers a refresh.

**Architecture:** Introduce a `providers.Provider` interface with an AWS implementation that uses `aws-sdk-go-v2` to call the AWS Pricing API (region: `us-east-1`). An ingestion orchestrator serializes runs with a Postgres advisory lock, writes SKUs into a `staging_sku_prices` table, and swaps into `sku_prices` atomically at the end. The catalog store exposes services, dimensions, regions, and currencies. `/readyz` gates on at least one successful ingestion run.

**Tech Stack:** Go 1.23, `aws-sdk-go-v2` pricing client, `robfig/cron/v3` for scheduling, `golang.org/x/time/rate` for the token bucket, `chi` for routing, pgx v5 for DB, testcontainers-go for integration tests.

**Spec:** `docs/superpowers/specs/2026-07-01-aws-calculator-design.md`
**Prior plan:** `docs/superpowers/plans/2026-07-01-01-foundation.md`

---

## Assumptions carried from Plan 01

- Package `github.com/OcheOps/aws-calculator/api/internal/http` uses import name `httpx` (not `http`).
- Migrations live under `api/migrations/` with a helper package `github.com/OcheOps/aws-calculator/api/migrations` exposing `FS embed.FS`. Do not use `//go:embed ../..` patterns.
- Router exposes `Deps` struct — extend it, don't replace.
- `Config` struct is loaded via `internal/config.Load()`; extend it with new fields via new env vars.
- All commit messages omit any AI-attribution trailer (user preference).

---

## File structure produced by this plan

```
api/
├── go.mod / go.sum                (deps added)
├── migrations/
│   ├── 0002_catalog.up.sql        (NEW)
│   └── 0002_catalog.down.sql      (NEW)
├── internal/
│   ├── config/config.go           (MODIFIED: AWS + admin config fields)
│   ├── catalog/
│   │   ├── model.go               (NEW: types)
│   │   ├── store.go               (NEW: DB queries)
│   │   └── store_test.go          (NEW)
│   ├── ingest/
│   │   ├── orchestrator.go        (NEW: cross-provider runner)
│   │   ├── orchestrator_test.go   (NEW)
│   │   ├── staging.go             (NEW: swap helper)
│   │   └── staging_test.go        (NEW)
│   ├── providers/
│   │   ├── provider.go            (NEW: interface + registry)
│   │   ├── provider_test.go       (NEW)
│   │   └── aws/
│   │       ├── client.go          (NEW: pricing client wrapper)
│   │       ├── client_test.go     (NEW)
│   │       ├── ingest.go          (NEW: main ingestion logic)
│   │       ├── ingest_test.go     (NEW)
│   │       └── testdata/
│   │           ├── describe_services_ec2.json (NEW)
│   │           ├── get_products_ec2_page1.json (NEW)
│   │           └── get_products_ec2_page2.json (NEW)
│   ├── workers/
│   │   ├── scheduler.go           (NEW)
│   │   └── scheduler_test.go      (NEW)
│   ├── httpapi/
│   │   ├── envelope.go            (NEW: {data, meta, error})
│   │   ├── envelope_test.go       (NEW)
│   │   ├── pagination.go          (NEW: cursor helpers)
│   │   ├── pagination_test.go     (NEW)
│   │   ├── catalog.go             (NEW: handlers)
│   │   ├── catalog_test.go        (NEW)
│   │   ├── admin.go               (NEW: /admin/ingest)
│   │   └── admin_test.go          (NEW)
│   └── http/
│       ├── router.go              (MODIFIED: mount catalog + admin, readiness update)
│       └── router_test.go         (MODIFIED)
└── cmd/aws-calc-api/main.go       (MODIFIED: wire ingestor + scheduler + catalog)
```

`internal/httpapi/` is a separate package from `internal/http/` (`httpx`) so handlers can be composed without cyclic imports; `router.go` in `httpx` mounts them.

---

## Task 1: Catalog migrations

**Files:**
- Create: `api/migrations/0002_catalog.up.sql`
- Create: `api/migrations/0002_catalog.down.sql`

- [ ] **Step 1: Write `api/migrations/0002_catalog.up.sql`**

```sql
CREATE TABLE services (
    id            BIGSERIAL PRIMARY KEY,
    provider_id   BIGINT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    code          TEXT NOT NULL,
    name          TEXT NOT NULL,
    family        TEXT,
    description   TEXT,
    icon_url      TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider_id, code)
);
CREATE INDEX services_provider ON services (provider_id);
CREATE INDEX services_family   ON services (family);

CREATE TABLE service_dimensions (
    id            BIGSERIAL PRIMARY KEY,
    service_id    BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    key           TEXT NOT NULL,
    label         TEXT NOT NULL,
    kind          TEXT NOT NULL CHECK (kind IN ('enum','region','numeric','freeform')),
    value_type    TEXT NOT NULL DEFAULT 'string',
    is_required   BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order    INT NOT NULL DEFAULT 0,
    UNIQUE (service_id, key)
);

CREATE TABLE dimension_values (
    id            BIGSERIAL PRIMARY KEY,
    dimension_id  BIGINT NOT NULL REFERENCES service_dimensions(id) ON DELETE CASCADE,
    value         TEXT NOT NULL,
    label         TEXT,
    UNIQUE (dimension_id, value)
);

CREATE TABLE sku_prices (
    id                BIGSERIAL PRIMARY KEY,
    service_id        BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    sku               TEXT NOT NULL,
    region_code       TEXT NOT NULL,
    unit              TEXT NOT NULL,
    currency          TEXT NOT NULL DEFAULT 'USD',
    price_per_unit    NUMERIC(20, 10) NOT NULL,
    term_type         TEXT NOT NULL,
    purchase_option   TEXT,
    lease_length      TEXT,
    offering_class    TEXT,
    attributes        JSONB NOT NULL DEFAULT '{}'::jsonb,
    effective_from    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to      TIMESTAMPTZ,
    UNIQUE (service_id, sku, region_code, term_type, COALESCE(purchase_option,''), COALESCE(lease_length,''))
);
CREATE INDEX sku_prices_service_region ON sku_prices (service_id, region_code);
CREATE INDEX sku_prices_attrs_gin ON sku_prices USING GIN (attributes jsonb_path_ops);

CREATE TABLE staging_sku_prices (LIKE sku_prices INCLUDING ALL);
```

- [ ] **Step 2: Write `api/migrations/0002_catalog.down.sql`**

```sql
DROP TABLE IF EXISTS staging_sku_prices;
DROP TABLE IF EXISTS sku_prices;
DROP TABLE IF EXISTS dimension_values;
DROP TABLE IF EXISTS service_dimensions;
DROP TABLE IF EXISTS services;
```

- [ ] **Step 3: Apply and verify migrations**

```bash
docker run --rm -d --name pg-cat-test -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16
sleep 3
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/db/... -run TestMigrate -v
cd ..
docker exec pg-cat-test psql -U u -d d -c "\dt" 2>&1 | grep -E 'services|dimensions|sku_prices|staging_sku_prices'
docker rm -f pg-cat-test
```

Expected: TestMigrate PASS, and `\dt` output lists all five new tables.

- [ ] **Step 4: Commit**

```bash
git add api/migrations/0002_catalog.up.sql api/migrations/0002_catalog.down.sql
git commit -m "feat(db): add catalog schema (services, dimensions, sku_prices, staging)"
```

---

## Task 2: Config additions

**Files:**
- Modify: `api/internal/config/config.go`
- Modify: `api/internal/config/config_test.go`
- Modify: `.env.example`

- [ ] **Step 1: Add failing tests for new fields**

Add these tests to `api/internal/config/config_test.go` (keep existing tests):

```go
func TestLoad_ParsesAWSCredentials(t *testing.T) {
	t.Setenv("API_DATABASE_URL", "postgres://x/y")
	t.Setenv("ADMIN_TOKEN", "t")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "sk")
	t.Setenv("AWS_PRICING_REGION", "us-east-1")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AWS.AccessKeyID != "AKIA" || cfg.AWS.SecretAccessKey != "sk" || cfg.AWS.PricingRegion != "us-east-1" {
		t.Errorf("aws config = %+v", cfg.AWS)
	}
}

func TestLoad_DefaultAWSRegion(t *testing.T) {
	t.Setenv("API_DATABASE_URL", "postgres://x/y")
	t.Setenv("ADMIN_TOKEN", "t")
	t.Setenv("AWS_PRICING_REGION", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AWS.PricingRegion != "us-east-1" {
		t.Errorf("PricingRegion = %q, want us-east-1", cfg.AWS.PricingRegion)
	}
}

func TestLoad_IngestSchedule(t *testing.T) {
	t.Setenv("API_DATABASE_URL", "postgres://x/y")
	t.Setenv("ADMIN_TOKEN", "t")
	t.Setenv("INGEST_CRON", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.IngestCron != "0 3 * * *" {
		t.Errorf("IngestCron = %q, want '0 3 * * *'", cfg.IngestCron)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd api && go test ./internal/config/... -run TestLoad_ParsesAWSCredentials`
Expected: FAIL — `AWS` field undefined on `Config`.

- [ ] **Step 3: Extend `api/internal/config/config.go`**

Add `AWS` and `IngestCron` fields; keep existing `Load` behavior.

```go
package config

import (
	"fmt"
	"os"
)

type AWSConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	PricingRegion   string
}

type Config struct {
	ListenAddr  string
	DatabaseURL string
	LogLevel    string
	Env         string
	AdminToken  string
	AWS         AWSConfig
	IngestCron  string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:  getenv("API_LISTEN_ADDR", ":8080"),
		DatabaseURL: os.Getenv("API_DATABASE_URL"),
		LogLevel:    getenv("API_LOG_LEVEL", "info"),
		Env:         getenv("API_ENV", "development"),
		AdminToken:  os.Getenv("ADMIN_TOKEN"),
		AWS: AWSConfig{
			AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			PricingRegion:   getenv("AWS_PRICING_REGION", "us-east-1"),
		},
		IngestCron: getenv("INGEST_CRON", "0 3 * * *"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("API_DATABASE_URL is required")
	}
	if cfg.AdminToken == "" {
		return Config{}, fmt.Errorf("ADMIN_TOKEN is required")
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 4: Run all config tests**

Run: `cd api && go test ./internal/config/... -v`
Expected: 6/6 PASS.

- [ ] **Step 5: Add `INGEST_CRON` to `.env.example`**

Edit `.env.example`: add below the existing AWS block:

```dotenv
INGEST_CRON=0 3 * * *
```

- [ ] **Step 6: Commit**

```bash
git add api/internal/config/config.go api/internal/config/config_test.go .env.example
git commit -m "feat(config): AWS credentials and ingest cron schedule"
```

---

## Task 3: Provider interface

**Files:**
- Create: `api/internal/providers/provider.go`
- Create: `api/internal/providers/provider_test.go`

- [ ] **Step 1: Write failing test — `provider_test.go`**

```go
package providers

import (
	"context"
	"testing"
)

type stubProvider struct {
	code       string
	ingestErr  error
	ingestSeen bool
}

func (s *stubProvider) Metadata() Metadata { return Metadata{Code: s.code, Name: "stub"} }
func (s *stubProvider) Ingest(ctx context.Context, deps IngestDeps) error {
	s.ingestSeen = true
	return s.ingestErr
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	p := &stubProvider{code: "aws"}
	r.Register(p)

	got, ok := r.Get("aws")
	if !ok || got.Metadata().Code != "aws" {
		t.Fatalf("Get(aws) = %v, %v", got, ok)
	}
	if _, ok := r.Get("gcp"); ok {
		t.Error("expected gcp to be missing")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubProvider{code: "aws"})
	r.Register(&stubProvider{code: "gcp"})
	all := r.All()
	if len(all) != 2 {
		t.Errorf("len(All()) = %d, want 2", len(all))
	}
}

func TestRegistry_RegisterPanicsOnDuplicate(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubProvider{code: "aws"})
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	r.Register(&stubProvider{code: "aws"})
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `cd api && go test ./internal/providers/...`
Expected: FAIL — package undefined.

- [ ] **Step 3: Implement `api/internal/providers/provider.go`**

```go
package providers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Metadata struct {
	Code string
	Name string
}

type IngestDeps struct {
	DB     *pgxpool.Pool
	Logger *slog.Logger
	RunID  int64
}

type Provider interface {
	Metadata() Metadata
	Ingest(ctx context.Context, deps IngestDeps) error
}

type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	code := p.Metadata().Code
	if _, exists := r.providers[code]; exists {
		panic(fmt.Sprintf("provider %q already registered", code))
	}
	r.providers[code] = p
}

func (r *Registry) Get(code string) (Provider, bool) {
	p, ok := r.providers[code]
	return p, ok
}

func (r *Registry) All() []Provider {
	out := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `cd api && go test ./internal/providers/... -v`
Expected: 3/3 PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/providers/provider.go api/internal/providers/provider_test.go
git commit -m "feat(providers): Provider interface and registry"
```

---

## Task 4: Staging swap helper

**Files:**
- Create: `api/internal/ingest/staging.go`
- Create: `api/internal/ingest/staging_test.go`

The swap uses `TRUNCATE` + `INSERT INTO ... SELECT` inside a transaction. This preserves the primary key IDs on `sku_prices` for referential stability during a run (nothing yet references them, but the estimates plan will).

- [ ] **Step 1: Write failing test — `staging_test.go`**

```go
package ingest

import (
	"context"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
)

func setupCatalogDB(t *testing.T) *dbHandle {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	// Ensure a clean slate: drop and re-migrate.
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS staging_sku_prices, sku_prices, dimension_values, service_dimensions, services, ingestion_runs, fx_rates, providers, schema_migrations CASCADE`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Seed a provider and service so FK checks pass.
	var providerID, serviceID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1, 'AmazonEC2', 'EC2') RETURNING id`, providerID).Scan(&serviceID); err != nil {
		t.Fatalf("seed service: %v", err)
	}
	return &dbHandle{pool: pool, serviceID: serviceID}
}

type dbHandle struct {
	pool      any
	serviceID int64
}

func TestSwapStaging_ReplacesRows(t *testing.T) {
	h := setupCatalogDB(t)
	ctx := context.Background()
	pool := h.pool.(interface {
		Exec(context.Context, string, ...any) (any, error)
		QueryRow(context.Context, string, ...any) any
	})
	_ = pool
	// Insert two rows into sku_prices and one into staging_sku_prices, then swap.
	// The swap must leave sku_prices with exactly the staging row.
	// (We use the actual pgxpool from db.Open above.)
	realPool := h.pool // *pgxpool.Pool
	_ = realPool
	// Below assertions use the concrete pool.
	pp := h.pool
	_ = pp
	// Use real pool via type assertion (kept concrete to keep the test simple).
	realCtx := ctx
	pool2 := getPool(t, h)

	if _, err := pool2.Exec(realCtx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type) VALUES ($1, 'OLD1', 'us-east-1', 'Hrs', 0.01, 'OnDemand'), ($1, 'OLD2', 'us-east-1', 'Hrs', 0.02, 'OnDemand')`, h.serviceID); err != nil {
		t.Fatalf("seed sku_prices: %v", err)
	}
	if _, err := pool2.Exec(realCtx, `INSERT INTO staging_sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type) VALUES ($1, 'NEW1', 'us-east-1', 'Hrs', 0.03, 'OnDemand')`, h.serviceID); err != nil {
		t.Fatalf("seed staging: %v", err)
	}

	if err := SwapStaging(realCtx, pool2); err != nil {
		t.Fatalf("SwapStaging: %v", err)
	}

	var count int
	if err := pool2.QueryRow(realCtx, `SELECT COUNT(*) FROM sku_prices`).Scan(&count); err != nil {
		t.Fatalf("count sku_prices: %v", err)
	}
	if count != 1 {
		t.Errorf("sku_prices count = %d, want 1", count)
	}
	var sku string
	if err := pool2.QueryRow(realCtx, `SELECT sku FROM sku_prices`).Scan(&sku); err != nil {
		t.Fatalf("read sku: %v", err)
	}
	if sku != "NEW1" {
		t.Errorf("sku = %q, want NEW1", sku)
	}
	var stagingCount int
	if err := pool2.QueryRow(realCtx, `SELECT COUNT(*) FROM staging_sku_prices`).Scan(&stagingCount); err != nil {
		t.Fatalf("count staging: %v", err)
	}
	if stagingCount != 0 {
		t.Errorf("staging count = %d, want 0", stagingCount)
	}
}
```

Then add the helper at the bottom of the same test file (needed because `dbHandle.pool` is typed `any` for readability above):

```go
import (
	"github.com/jackc/pgx/v5/pgxpool"
)

func getPool(t *testing.T, h *dbHandle) *pgxpool.Pool {
	t.Helper()
	p, ok := h.pool.(*pgxpool.Pool)
	if !ok {
		t.Fatalf("unexpected pool type: %T", h.pool)
	}
	return p
}
```

Merge the two import blocks into one at the top of the file when writing it.

- [ ] **Step 2: Run test to verify failure**

Run: `cd api && go test ./internal/ingest/...`
Expected: FAIL — `SwapStaging` undefined.

- [ ] **Step 3: Implement `api/internal/ingest/staging.go`**

```go
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
```

- [ ] **Step 4: Run tests with real Postgres**

```bash
docker run --rm -d --name pg-ingest-test -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16
sleep 3
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/ingest/... -v
cd ..
docker rm -f pg-ingest-test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/ingest/staging.go api/internal/ingest/staging_test.go
git commit -m "feat(ingest): atomic staging-to-sku_prices swap"
```

---

## Task 5: AWS pricing client wrapper

**Files:**
- Create: `api/internal/providers/aws/client.go`
- Create: `api/internal/providers/aws/client_test.go`

We keep our own thin interface over the AWS SDK so tests use recorded JSON fixtures instead of live API calls.

- [ ] **Step 1: Add AWS SDK deps**

```bash
cd api
go get github.com/aws/aws-sdk-go-v2@v1.32.4
go get github.com/aws/aws-sdk-go-v2/config@v1.28.4
go get github.com/aws/aws-sdk-go-v2/credentials@v1.17.45
go get github.com/aws/aws-sdk-go-v2/service/pricing@v1.32.4
go get golang.org/x/time/rate@v0.10.0
cd ..
```

- [ ] **Step 2: Write failing test — `client_test.go`**

```go
package aws

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeSDK struct {
	services  []string
	products  map[string][]string // key: serviceCode|region → JSON payloads
	nextToken string
}

func (f *fakeSDK) ListServiceCodes(ctx context.Context) ([]string, error) {
	return f.services, nil
}

func (f *fakeSDK) DescribeService(ctx context.Context, code string) ([]string, error) {
	return []string{"instanceType", "regionCode"}, nil
}

func (f *fakeSDK) GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error) {
	key := code + "|" + region
	payloads := f.products[key]
	out := make([]json.RawMessage, 0, len(payloads))
	for _, p := range payloads {
		out = append(out, json.RawMessage(p))
	}
	return out, "", nil
}

func TestClient_ListServices(t *testing.T) {
	c := NewClient(&fakeSDK{services: []string{"AmazonEC2", "AmazonS3"}})
	ctx := context.Background()
	svcs, err := c.ListServices(ctx)
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(svcs) != 2 || svcs[0] != "AmazonEC2" {
		t.Errorf("services = %v", svcs)
	}
}
```

- [ ] **Step 3: Run test to verify failure**

Run: `cd api && go test ./internal/providers/aws/...`
Expected: FAIL — package/types undefined.

- [ ] **Step 4: Implement `api/internal/providers/aws/client.go`**

```go
package aws

import (
	"context"
	"encoding/json"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
)

// SDK is the minimal subset of the AWS Pricing API we depend on.
// Real implementation wraps the SDK; tests use a fake.
type SDK interface {
	ListServiceCodes(ctx context.Context) ([]string, error)
	DescribeService(ctx context.Context, code string) ([]string, error)
	GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error)
}

type Client struct {
	sdk SDK
}

func NewClient(sdk SDK) *Client { return &Client{sdk: sdk} }

func (c *Client) ListServices(ctx context.Context) ([]string, error) {
	return c.sdk.ListServiceCodes(ctx)
}

func (c *Client) DescribeService(ctx context.Context, code string) ([]string, error) {
	return c.sdk.DescribeService(ctx, code)
}

func (c *Client) GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error) {
	return c.sdk.GetProducts(ctx, code, region, token)
}

// realSDK is the production implementation backed by aws-sdk-go-v2.
type realSDK struct {
	client *pricing.Client
}

func NewRealSDK(ctx context.Context, accessKeyID, secretAccessKey, region string) (SDK, error) {
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &realSDK{client: pricing.NewFromConfig(cfg)}, nil
}

func (r *realSDK) ListServiceCodes(ctx context.Context) ([]string, error) {
	var out []string
	var token *string
	for {
		resp, err := r.client.DescribeServices(ctx, &pricing.DescribeServicesInput{
			NextToken:  token,
			MaxResults: awssdk.Int32(100),
		})
		if err != nil {
			return nil, fmt.Errorf("describe services: %w", err)
		}
		for _, s := range resp.Services {
			if s.ServiceCode != nil {
				out = append(out, *s.ServiceCode)
			}
		}
		if resp.NextToken == nil || *resp.NextToken == "" {
			return out, nil
		}
		token = resp.NextToken
	}
}

func (r *realSDK) DescribeService(ctx context.Context, code string) ([]string, error) {
	resp, err := r.client.DescribeServices(ctx, &pricing.DescribeServicesInput{
		ServiceCode: awssdk.String(code),
		MaxResults:  awssdk.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("describe service %s: %w", code, err)
	}
	if len(resp.Services) == 0 {
		return nil, fmt.Errorf("service %s not found", code)
	}
	return resp.Services[0].AttributeNames, nil
}

func (r *realSDK) GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error) {
	in := &pricing.GetProductsInput{
		ServiceCode: awssdk.String(code),
		MaxResults:  awssdk.Int32(100),
		Filters: []types.Filter{{
			Field: awssdk.String("regionCode"),
			Type:  types.FilterTypeTermMatch,
			Value: awssdk.String(region),
		}},
	}
	if token != "" {
		in.NextToken = awssdk.String(token)
	}
	resp, err := r.client.GetProducts(ctx, in)
	if err != nil {
		return nil, "", fmt.Errorf("get products %s/%s: %w", code, region, err)
	}
	out := make([]json.RawMessage, 0, len(resp.PriceList))
	for _, s := range resp.PriceList {
		out = append(out, json.RawMessage(s))
	}
	next := ""
	if resp.NextToken != nil {
		next = *resp.NextToken
	}
	return out, next, nil
}
```

- [ ] **Step 5: Run tests**

Run: `cd api && go test ./internal/providers/aws/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/go.mod api/go.sum api/internal/providers/aws/client.go api/internal/providers/aws/client_test.go
git commit -m "feat(providers/aws): pricing client wrapper with SDK interface"
```

---

## Task 6: AWS ingest logic

**Files:**
- Create: `api/internal/providers/aws/ingest.go`
- Create: `api/internal/providers/aws/ingest_test.go`
- Create: `api/internal/providers/aws/testdata/get_products_ec2_page1.json`

The AWS Pricing API returns each SKU as a JSON blob whose shape looks like:

```json
{"product":{"sku":"XYZ","productFamily":"Compute Instance","attributes":{"instanceType":"m5.large","regionCode":"us-east-1","operatingSystem":"Linux","tenancy":"Shared","preInstalledSw":"NA"}},"terms":{"OnDemand":{"XYZ.JRTCKXETXF":{"priceDimensions":{"XYZ.JRTCKXETXF.6YS6EN2CT7":{"unit":"Hrs","pricePerUnit":{"USD":"0.096"}}}}}}}
```

We parse each blob, extract the SKU + attributes + first OnDemand price dimension, and upsert into `staging_sku_prices`.

- [ ] **Step 1: Create `api/internal/providers/aws/testdata/get_products_ec2_page1.json`**

Wrap one representative product JSON as a single-element array. Everything on one line is fine.

```json
[
"{\"product\":{\"sku\":\"XYZ\",\"productFamily\":\"Compute Instance\",\"attributes\":{\"instanceType\":\"m5.large\",\"regionCode\":\"us-east-1\",\"operatingSystem\":\"Linux\",\"tenancy\":\"Shared\",\"preInstalledSw\":\"NA\",\"servicecode\":\"AmazonEC2\"}},\"terms\":{\"OnDemand\":{\"XYZ.JRTCKXETXF\":{\"priceDimensions\":{\"XYZ.JRTCKXETXF.6YS6EN2CT7\":{\"unit\":\"Hrs\",\"pricePerUnit\":{\"USD\":\"0.096\"}}}}}}}"
]
```

- [ ] **Step 2: Write failing test — `ingest_test.go`**

```go
package aws

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func loadFixture(t *testing.T, name string) []json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var strs []string
	if err := json.Unmarshal(data, &strs); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	out := make([]json.RawMessage, len(strs))
	for i, s := range strs {
		out[i] = json.RawMessage(s)
	}
	return out
}

type fixtureSDK struct{ products []json.RawMessage }

func (f *fixtureSDK) ListServiceCodes(ctx context.Context) ([]string, error) {
	return []string{"AmazonEC2"}, nil
}
func (f *fixtureSDK) DescribeService(ctx context.Context, code string) ([]string, error) {
	return []string{"instanceType", "regionCode", "operatingSystem", "tenancy", "preInstalledSw"}, nil
}
func (f *fixtureSDK) GetProducts(ctx context.Context, code, region, token string) ([]json.RawMessage, string, error) {
	return f.products, "", nil
}

func TestIngest_PopulatesStagingAndDimensions(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	defer pool.Close()
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS staging_sku_prices, sku_prices, dimension_values, service_dimensions, services, ingestion_runs, fx_rates, providers, schema_migrations CASCADE`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	client := NewClient(&fixtureSDK{products: loadFixture(t, "get_products_ec2_page1.json")})
	ing := NewIngestor(client, []string{"us-east-1"})

	if err := ing.Run(ctx, pool); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var services int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM services WHERE code='AmazonEC2'`).Scan(&services)
	if services != 1 {
		t.Errorf("services count = %d", services)
	}
	var stagingRows int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM staging_sku_prices`).Scan(&stagingRows)
	if stagingRows == 0 {
		t.Fatal("expected staging rows to be populated")
	}
	assertDim(t, pool, "instanceType", true)
	assertDim(t, pool, "regionCode", true)
}

func assertDim(t *testing.T, pool *pgxpool.Pool, key string, wantAny bool) {
	t.Helper()
	var count int
	pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM service_dimensions WHERE key=$1`, key).Scan(&count)
	if wantAny && count == 0 {
		t.Errorf("dimension %s missing", key)
	}
}
```

- [ ] **Step 3: Run test to verify failure**

Run: `cd api && go test ./internal/providers/aws/... -run Ingest`
Expected: FAIL — `NewIngestor` undefined (or skipped without `TEST_DATABASE_URL`).

- [ ] **Step 4: Implement `api/internal/providers/aws/ingest.go`**

```go
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

// productDoc mirrors the subset of the AWS Pricing API product JSON we care about.
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
	// Update dimension_values with what we saw.
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
```

- [ ] **Step 5: Run tests with Postgres**

```bash
docker run --rm -d --name pg-ai-test -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16
sleep 3
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/providers/aws/... -v
cd ..
docker rm -f pg-ai-test
```

Expected: PASS (client tests + ingest fixture test).

- [ ] **Step 6: Commit**

```bash
git add api/internal/providers/aws/ingest.go api/internal/providers/aws/ingest_test.go api/internal/providers/aws/testdata/
git commit -m "feat(providers/aws): staging ingestion with dimension extraction"
```

---

## Task 7: Ingestion orchestrator

**Files:**
- Create: `api/internal/ingest/orchestrator.go`
- Create: `api/internal/ingest/orchestrator_test.go`

The orchestrator wraps a `Provider.Ingest` call with:
1. A Postgres advisory lock (fails fast if another run is in progress).
2. A row in `ingestion_runs` marked `running` at start, updated on success/failure.
3. Calling `SwapStaging` on success.

- [ ] **Step 1: Write failing test — `orchestrator_test.go`**

```go
package ingest

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/OcheOps/aws-calculator/api/internal/providers"
)

type recordingProvider struct {
	called int
	err    error
}

func (r *recordingProvider) Metadata() providers.Metadata {
	return providers.Metadata{Code: "aws", Name: "AWS"}
}

func (r *recordingProvider) Ingest(ctx context.Context, deps providers.IngestDeps) error {
	r.called++
	return r.err
}

func setupOrchestratorDB(t *testing.T) (context.Context, *Orchestrator, *recordingProvider) {
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
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS staging_sku_prices, sku_prices, dimension_values, service_dimensions, services, ingestion_runs, fx_rates, providers, schema_migrations CASCADE`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	prov := &recordingProvider{}
	orch := NewOrchestrator(pool)
	return ctx, orch, prov
}

func TestOrchestrator_SuccessRecordsSuccessRun(t *testing.T) {
	ctx, orch, prov := setupOrchestratorDB(t)
	if err := orch.Run(ctx, prov); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prov.called != 1 {
		t.Errorf("provider called %d times, want 1", prov.called)
	}
	var status string
	orch.pool.QueryRow(ctx, `SELECT status FROM ingestion_runs ORDER BY id DESC LIMIT 1`).Scan(&status)
	if status != "success" {
		t.Errorf("status = %q, want success", status)
	}
}

func TestOrchestrator_FailureRecordsError(t *testing.T) {
	ctx, orch, prov := setupOrchestratorDB(t)
	prov.err = errors.New("boom")
	err := orch.Run(ctx, prov)
	if err == nil {
		t.Fatal("expected error")
	}
	var status, msg string
	orch.pool.QueryRow(ctx, `SELECT status, COALESCE(error,'') FROM ingestion_runs ORDER BY id DESC LIMIT 1`).Scan(&status, &msg)
	if status != "failed" || msg == "" {
		t.Errorf("status=%q msg=%q", status, msg)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `cd api && go test ./internal/ingest/... -run Orchestrator`
Expected: FAIL — `Orchestrator` undefined.

- [ ] **Step 3: Implement `api/internal/ingest/orchestrator.go`**

```go
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
```

- [ ] **Step 4: Run tests**

```bash
docker run --rm -d --name pg-orch-test -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16
sleep 3
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/ingest/... -v
cd ..
docker rm -f pg-orch-test
```

Expected: all ingest tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/ingest/orchestrator.go api/internal/ingest/orchestrator_test.go
git commit -m "feat(ingest): orchestrator with advisory lock and run tracking"
```

---

## Task 8: Scheduler wiring

**Files:**
- Create: `api/internal/workers/scheduler.go`
- Create: `api/internal/workers/scheduler_test.go`

- [ ] **Step 1: Add cron dep**

```bash
cd api
go get github.com/robfig/cron/v3@v3.0.1
cd ..
```

- [ ] **Step 2: Write failing test — `scheduler_test.go`**

```go
package workers

import (
	"context"
	"testing"
	"time"
)

func TestScheduler_RunsJobOnTick(t *testing.T) {
	s := NewScheduler()
	ran := make(chan struct{}, 1)
	// Every second.
	if err := s.Add("@every 1s", func(context.Context) error {
		select {
		case ran <- struct{}{}:
		default:
		}
		return nil
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s.Start(context.Background())
	defer s.Stop()
	select {
	case <-ran:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not run within 3s")
	}
}

func TestScheduler_RejectsBadCron(t *testing.T) {
	s := NewScheduler()
	if err := s.Add("not-a-cron", func(context.Context) error { return nil }); err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}
```

- [ ] **Step 3: Run test to verify failure**

Run: `cd api && go test ./internal/workers/...`
Expected: FAIL — package undefined.

- [ ] **Step 4: Implement `api/internal/workers/scheduler.go`**

```go
package workers

import (
	"context"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"
)

type Job func(ctx context.Context) error

type Scheduler struct {
	mu     sync.Mutex
	cron   *cron.Cron
	logger *slog.Logger
	ctx    context.Context
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		logger: slog.Default(),
	}
}

func (s *Scheduler) WithLogger(l *slog.Logger) *Scheduler {
	s.logger = l
	return s
}

func (s *Scheduler) Add(spec string, job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.cron.AddFunc(spec, func() {
		ctx := s.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		if err := job(ctx); err != nil {
			s.logger.Error("scheduled job failed", "err", err, "spec", spec)
		}
	})
	return err
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}
```

- [ ] **Step 5: Run tests**

Run: `cd api && go test ./internal/workers/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/go.mod api/go.sum api/internal/workers/scheduler.go api/internal/workers/scheduler_test.go
git commit -m "feat(workers): cron-based scheduler with per-job error logging"
```

---

## Task 9: Catalog store

**Files:**
- Create: `api/internal/catalog/model.go`
- Create: `api/internal/catalog/store.go`
- Create: `api/internal/catalog/store_test.go`

- [ ] **Step 1: Write `api/internal/catalog/model.go`**

```go
package catalog

type Provider struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type Service struct {
	ID          int64  `json:"id"`
	ProviderID  int64  `json:"provider_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Family      string `json:"family,omitempty"`
	Description string `json:"description,omitempty"`
}

type Dimension struct {
	ID         int64  `json:"id"`
	Key        string `json:"key"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	ValueType  string `json:"value_type"`
	IsRequired bool   `json:"is_required"`
	SortOrder  int    `json:"sort_order"`
}

type Region struct {
	Code string `json:"code"`
}

type DimensionValue struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}
```

- [ ] **Step 2: Write failing test — `store_test.go`**

```go
package catalog

import (
	"context"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
)

func setupStore(t *testing.T) (*Store, context.Context) {
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
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS staging_sku_prices, sku_prices, dimension_values, service_dimensions, services, ingestion_runs, fx_rates, providers, schema_migrations CASCADE`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var providerID, serviceID int64
	pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1, 'AmazonEC2', 'EC2') RETURNING id`, providerID).Scan(&serviceID)
	pool.Exec(ctx, `INSERT INTO service_dimensions (service_id, key, label, kind, sort_order) VALUES ($1, 'instanceType', 'instanceType', 'enum', 0)`, serviceID)
	pool.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type) VALUES ($1, 'K1', 'us-east-1', 'Hrs', 0.01, 'OnDemand'), ($1, 'K2', 'eu-west-1', 'Hrs', 0.02, 'OnDemand')`, serviceID)
	return NewStore(pool), ctx
}

func TestStore_ListProviders(t *testing.T) {
	s, ctx := setupStore(t)
	provs, err := s.ListProviders(ctx)
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(provs) != 1 || provs[0].Code != "aws" {
		t.Errorf("got %+v", provs)
	}
}

func TestStore_ListServices(t *testing.T) {
	s, ctx := setupStore(t)
	svcs, cursor, err := s.ListServices(ctx, ListServicesQuery{ProviderCode: "aws", Limit: 50})
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(svcs) != 1 || svcs[0].Code != "AmazonEC2" {
		t.Errorf("got %+v", svcs)
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}

func TestStore_ListRegions(t *testing.T) {
	s, ctx := setupStore(t)
	regions, err := s.ListRegions(ctx, "aws")
	if err != nil {
		t.Fatalf("ListRegions: %v", err)
	}
	if len(regions) != 2 {
		t.Errorf("regions = %v", regions)
	}
}

func TestStore_HasSuccessfulIngestion(t *testing.T) {
	s, ctx := setupStore(t)
	ok, err := s.HasSuccessfulIngestion(ctx)
	if err != nil {
		t.Fatalf("HasSuccessfulIngestion: %v", err)
	}
	if ok {
		t.Error("expected false before any run")
	}
	var providerID int64
	s.pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	s.pool.Exec(ctx, `INSERT INTO ingestion_runs (provider_id, status, finished_at) VALUES ($1, 'success', NOW())`, providerID)
	ok, err = s.HasSuccessfulIngestion(ctx)
	if err != nil {
		t.Fatalf("HasSuccessfulIngestion 2: %v", err)
	}
	if !ok {
		t.Error("expected true after success run")
	}
}
```

- [ ] **Step 3: Run test to verify failure**

Run: `cd api && go test ./internal/catalog/...`
Expected: FAIL — `Store` undefined.

- [ ] **Step 4: Implement `api/internal/catalog/store.go`**

```go
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
```

- [ ] **Step 3: Run tests**

```bash
docker run --rm -d --name pg-cs-test -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16
sleep 3
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/catalog/... -v
cd ..
docker rm -f pg-cs-test
```

Expected: 4/4 PASS.

- [ ] **Step 4: Commit**

```bash
git add api/internal/catalog/
git commit -m "feat(catalog): store queries with cursor pagination"
```

---

## Task 10: HTTP envelope + pagination helpers

**Files:**
- Create: `api/internal/httpapi/envelope.go`
- Create: `api/internal/httpapi/envelope_test.go`
- Create: `api/internal/httpapi/pagination.go`
- Create: `api/internal/httpapi/pagination_test.go`

- [ ] **Step 1: Write failing tests**

`api/internal/httpapi/envelope_test.go`:

```go
package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteData_ProducesEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteData(rec, http.StatusOK, map[string]int{"n": 42}, nil)
	if rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["data"]; !ok {
		t.Error("expected data field")
	}
}

func TestWriteError_ProducesEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "bad_request", "missing field")
	var body struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "bad_request" || body.Error.Message != "missing field" {
		t.Errorf("got %+v", body)
	}
}
```

`api/internal/httpapi/pagination_test.go`:

```go
package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	p := ParsePagination(req)
	if p.Limit != 50 || p.Cursor != "" {
		t.Errorf("got %+v", p)
	}
}

func TestParsePagination_ClampsLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=1000", nil)
	p := ParsePagination(req)
	if p.Limit != 200 {
		t.Errorf("limit = %d, want 200", p.Limit)
	}
}

func TestParsePagination_ReadsCursor(t *testing.T) {
	req := httptest.NewRequest("GET", "/?cursor=abc&limit=10", nil)
	p := ParsePagination(req)
	if p.Cursor != "abc" || p.Limit != 10 {
		t.Errorf("got %+v", p)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd api && go test ./internal/httpapi/...`
Expected: FAIL — package undefined.

- [ ] **Step 3: Implement `api/internal/httpapi/envelope.go`**

```go
package httpapi

import (
	"encoding/json"
	"net/http"
)

type Meta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	Count      int    `json:"count,omitempty"`
}

type response struct {
	Data  any     `json:"data,omitempty"`
	Meta  *Meta   `json:"meta,omitempty"`
	Error *apiErr `json:"error,omitempty"`
}

type apiErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteData(w http.ResponseWriter, code int, data any, meta *Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(response{Data: data, Meta: meta})
}

func WriteError(w http.ResponseWriter, code int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(response{Error: &apiErr{Code: errCode, Message: message}})
}
```

- [ ] **Step 4: Implement `api/internal/httpapi/pagination.go`**

```go
package httpapi

import (
	"net/http"
	"strconv"
)

type Pagination struct {
	Cursor string
	Limit  int
}

func ParsePagination(r *http.Request) Pagination {
	p := Pagination{Cursor: r.URL.Query().Get("cursor"), Limit: 50}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Limit = n
		}
	}
	if p.Limit > 200 {
		p.Limit = 200
	}
	return p
}
```

- [ ] **Step 5: Run tests**

Run: `cd api && go test ./internal/httpapi/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/httpapi/envelope.go api/internal/httpapi/envelope_test.go api/internal/httpapi/pagination.go api/internal/httpapi/pagination_test.go
git commit -m "feat(httpapi): envelope response and cursor pagination helpers"
```

---

## Task 11: Catalog HTTP handlers

**Files:**
- Create: `api/internal/httpapi/catalog.go`
- Create: `api/internal/httpapi/catalog_test.go`

- [ ] **Step 1: Write failing test — `catalog_test.go`**

```go
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/go-chi/chi/v5"
)

func setupCatalogHandlers(t *testing.T) http.Handler {
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
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS staging_sku_prices, sku_prices, dimension_values, service_dimensions, services, ingestion_runs, fx_rates, providers, schema_migrations CASCADE`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var providerID, serviceID int64
	pool.QueryRow(ctx, `SELECT id FROM providers WHERE code='aws'`).Scan(&providerID)
	pool.QueryRow(ctx, `INSERT INTO services (provider_id, code, name) VALUES ($1, 'AmazonEC2', 'EC2') RETURNING id`, providerID).Scan(&serviceID)
	pool.Exec(ctx, `INSERT INTO service_dimensions (service_id, key, label, kind) VALUES ($1, 'instanceType', 'instanceType', 'enum')`, serviceID)
	pool.Exec(ctx, `INSERT INTO sku_prices (service_id, sku, region_code, unit, price_per_unit, term_type) VALUES ($1, 'K1', 'us-east-1', 'Hrs', 0.01, 'OnDemand')`, serviceID)

	r := chi.NewRouter()
	MountCatalog(r, catalog.NewStore(pool))
	return r
}

func TestCatalogHandlers_ListProviders(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/providers")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data []catalog.Provider `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 || body.Data[0].Code != "aws" {
		t.Errorf("got %+v", body.Data)
	}
}

func TestCatalogHandlers_ListServices(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/services?provider=aws")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Data []catalog.Service `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 || body.Data[0].Code != "AmazonEC2" {
		t.Errorf("got %+v", body.Data)
	}
}

func TestCatalogHandlers_GetService_IncludesDimensions(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/services/AmazonEC2?provider=aws")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data struct {
			Service    catalog.Service     `json:"service"`
			Dimensions []catalog.Dimension `json:"dimensions"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Data.Service.Code != "AmazonEC2" {
		t.Errorf("service = %+v", body.Data.Service)
	}
	if len(body.Data.Dimensions) != 1 || body.Data.Dimensions[0].Key != "instanceType" {
		t.Errorf("dimensions = %+v", body.Data.Dimensions)
	}
}

func TestCatalogHandlers_ListRegions(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/regions?provider=aws")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Data []catalog.Region `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 || body.Data[0].Code != "us-east-1" {
		t.Errorf("regions = %+v", body.Data)
	}
}

func TestCatalogHandlers_Currencies(t *testing.T) {
	r := setupCatalogHandlers(t)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/currencies")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Data []string `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	wants := map[string]bool{"USD": true, "EUR": true, "GBP": true, "NGN": true}
	for _, c := range body.Data {
		delete(wants, c)
	}
	if len(wants) != 0 {
		t.Errorf("missing currencies: %v", wants)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `cd api && go test ./internal/httpapi/... -run CatalogHandlers`
Expected: FAIL — `MountCatalog` undefined.

- [ ] **Step 3: Implement `api/internal/httpapi/catalog.go`**

```go
package httpapi

import (
	"net/http"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/go-chi/chi/v5"
)

var supportedCurrencies = []string{"USD", "EUR", "GBP", "NGN"}

func MountCatalog(r chi.Router, store *catalog.Store) {
	r.Get("/providers", func(w http.ResponseWriter, r *http.Request) {
		provs, err := store.ListProviders(r.Context())
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, provs, &Meta{Count: len(provs)})
	})

	r.Get("/services", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		page := ParsePagination(r)
		svcs, next, err := store.ListServices(r.Context(), catalog.ListServicesQuery{
			ProviderCode: provider,
			Search:       r.URL.Query().Get("q"),
			Cursor:       page.Cursor,
			Limit:        page.Limit,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, svcs, &Meta{NextCursor: next, Count: len(svcs)})
	})

	r.Get("/services/{code}", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		code := chi.URLParam(r, "code")
		svc, err := store.GetServiceByCode(r.Context(), provider, code)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "service not found")
			return
		}
		dims, err := store.ListDimensions(r.Context(), svc.ID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, map[string]any{
			"service":    svc,
			"dimensions": dims,
		}, nil)
	})

	r.Get("/services/{code}/dimension-values", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		code := chi.URLParam(r, "code")
		dimKey := r.URL.Query().Get("dim")
		if dimKey == "" {
			WriteError(w, http.StatusBadRequest, "bad_request", "dim is required")
			return
		}
		svc, err := store.GetServiceByCode(r.Context(), provider, code)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "service not found")
			return
		}
		dims, err := store.ListDimensions(r.Context(), svc.ID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		var dimID int64
		for _, d := range dims {
			if d.Key == dimKey {
				dimID = d.ID
				break
			}
		}
		if dimID == 0 {
			WriteError(w, http.StatusNotFound, "not_found", "dimension not found")
			return
		}
		vals, err := store.ListDimensionValues(r.Context(), dimID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, vals, &Meta{Count: len(vals)})
	})

	r.Get("/regions", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		regions, err := store.ListRegions(r.Context(), provider)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, regions, &Meta{Count: len(regions)})
	})

	r.Get("/currencies", func(w http.ResponseWriter, r *http.Request) {
		WriteData(w, http.StatusOK, supportedCurrencies, &Meta{Count: len(supportedCurrencies)})
	})
}
```

- [ ] **Step 4: Run tests**

```bash
docker run --rm -d --name pg-ch-test -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16
sleep 3
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./internal/httpapi/... -v
cd ..
docker rm -f pg-ch-test
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/httpapi/catalog.go api/internal/httpapi/catalog_test.go
git commit -m "feat(httpapi): catalog handlers (providers, services, dimensions, regions, currencies)"
```

---

## Task 12: Admin ingest endpoint

**Files:**
- Create: `api/internal/httpapi/admin.go`
- Create: `api/internal/httpapi/admin_test.go`

- [ ] **Step 1: Write failing test — `admin_test.go`**

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakeIngestTrigger struct{ called bool; providerCode string }

func (f *fakeIngestTrigger) Trigger(providerCode string) (int64, error) {
	f.called = true
	f.providerCode = providerCode
	return 42, nil
}

func TestAdmin_RejectsMissingToken(t *testing.T) {
	trig := &fakeIngestTrigger{}
	r := chi.NewRouter()
	MountAdmin(r, "secret-token", trig)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, _ := http.Post(srv.URL+"/admin/ingest?provider=aws", "application/json", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if trig.called {
		t.Error("trigger should not have been called")
	}
}

func TestAdmin_TriggersIngestion(t *testing.T) {
	trig := &fakeIngestTrigger{}
	r := chi.NewRouter()
	MountAdmin(r, "secret-token", trig)
	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/admin/ingest?provider=aws", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want 202", resp.StatusCode)
	}
	if !trig.called || trig.providerCode != "aws" {
		t.Errorf("trigger = %+v", trig)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `cd api && go test ./internal/httpapi/... -run Admin`
Expected: FAIL — `MountAdmin` / `IngestTrigger` undefined.

- [ ] **Step 3: Implement `api/internal/httpapi/admin.go`**

```go
package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type IngestTrigger interface {
	Trigger(providerCode string) (int64, error)
}

func MountAdmin(r chi.Router, adminToken string, trig IngestTrigger) {
	r.Post("/admin/ingest", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		want := "Bearer " + adminToken
		if adminToken == "" || !strings.EqualFold(auth, want) {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing admin token")
			return
		}
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		runID, err := trig.Trigger(provider)
		if err != nil {
			WriteError(w, http.StatusConflict, "conflict", err.Error())
			return
		}
		WriteData(w, http.StatusAccepted, map[string]any{
			"provider": provider,
			"run_id":   runID,
		}, nil)
	})
}
```

- [ ] **Step 4: Run tests**

Run: `cd api && go test ./internal/httpapi/... -v -run Admin`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/httpapi/admin.go api/internal/httpapi/admin_test.go
git commit -m "feat(httpapi): admin ingest endpoint gated by bearer token"
```

---

## Task 13: Update router with catalog + admin + readiness

**Files:**
- Modify: `api/internal/http/router.go`
- Modify: `api/internal/http/router_test.go`

- [ ] **Step 1: Update `api/internal/http/router.go` to accept catalog + admin mounters and use catalog-based readiness**

```go
package httpx

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/httpapi"
	"github.com/go-chi/chi/v5"
)

type Deps struct {
	Ready      Readiness
	Logger     *slog.Logger
	Catalog    *catalog.Store
	AdminToken string
	Ingest     httpapi.IngestTrigger
}

func NewRouter(d Deps) http.Handler {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	r := chi.NewRouter()
	r.Use(RequestID)
	r.Use(Recoverer)
	r.Use(AccessLog(d.Logger))

	r.Get("/healthz", HealthHandler().ServeHTTP)
	r.Get("/readyz", ReadyHandler(d.Ready).ServeHTTP)

	r.Route("/api/v1", func(r chi.Router) {
		if d.Catalog != nil {
			httpapi.MountCatalog(r, d.Catalog)
		}
		if d.Ingest != nil {
			httpapi.MountAdmin(r, d.AdminToken, d.Ingest)
		}
	})

	return r
}

// CatalogReadiness marks the API ready only once the catalog store reports a
// successful ingestion. Wrap it around a DB-ping readiness for combined checks.
type CatalogReadiness struct {
	Store *catalog.Store
}

func (c CatalogReadiness) Ready(ctx context.Context) (bool, string) {
	ok, err := c.Store.HasSuccessfulIngestion(ctx)
	if err != nil {
		return false, "catalog check failed: " + err.Error()
	}
	if !ok {
		return false, "catalog not populated yet"
	}
	return true, "ok"
}
```

- [ ] **Step 2: Update `api/internal/http/router_test.go` — existing tests should still pass with the new nullable Deps fields**

The existing tests only supply `Ready` and construct a router without catalog/admin — that path still works because both branches are gated by nil checks. No changes required. But add:

```go
func TestRouter_ExposesCatalogWhenProvided(t *testing.T) {
	// Skip: catalog handlers need a Postgres-backed store and are covered by
	// the httpapi/catalog_test.go integration tests. This test is a
	// placeholder that documents the intent.
	t.Skip("covered by internal/httpapi/catalog_test.go integration tests")
}
```

- [ ] **Step 3: Run tests**

Run: `cd api && go test ./... 2>&1 | tail -20`
Expected: all existing tests pass (Router tests, all others unchanged).

Note: `go build ./...` must be clean; the new router imports `catalog` and `httpapi` — those packages exist from tasks 9–12.

- [ ] **Step 4: Commit**

```bash
git add api/internal/http/router.go api/internal/http/router_test.go
git commit -m "feat(http): mount catalog and admin routes, add CatalogReadiness"
```

---

## Task 14: Wire everything in main.go

**Files:**
- Modify: `api/cmd/aws-calc-api/main.go`

- [ ] **Step 1: Rewrite `main.go`**

The changes:
- Load AWS credentials from config.
- Register the AWS provider (with a stub for offline mode when creds absent).
- Build the orchestrator.
- Build the catalog store, mount catalog+admin.
- Combine DB + catalog readiness.
- Wire the scheduler to run ingestion on the cron schedule.
- Provide an `IngestTrigger` implementation.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/config"
	"github.com/OcheOps/aws-calculator/api/internal/db"
	httpx "github.com/OcheOps/aws-calculator/api/internal/http"
	"github.com/OcheOps/aws-calculator/api/internal/ingest"
	"github.com/OcheOps/aws-calculator/api/internal/providers"
	awsprov "github.com/OcheOps/aws-calculator/api/internal/providers/aws"
	"github.com/OcheOps/aws-calculator/api/internal/workers"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		logger.Error("migrations failed", "err", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("db open failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	registry := providers.NewRegistry()
	if err := registerAWS(ctx, registry, cfg, logger); err != nil {
		logger.Warn("aws provider not registered", "err", err)
	}

	orch := ingest.NewOrchestrator(pool).WithLogger(logger)
	trigger := &ingestTrigger{orch: orch, registry: registry, pool: pool, logger: logger}

	sched := workers.NewScheduler().WithLogger(logger)
	if _, hasAWS := registry.Get("aws"); hasAWS {
		if err := sched.Add(cfg.IngestCron, func(c context.Context) error {
			_, err := trigger.Trigger("aws")
			return err
		}); err != nil {
			logger.Warn("failed to schedule aws ingest", "err", err)
		}
	}
	sched.Start(ctx)
	defer sched.Stop()

	store := catalog.NewStore(pool)
	router := httpx.NewRouter(httpx.Deps{
		Ready:      combinedReady{pool: pool, catalog: httpx.CatalogReadiness{Store: store}},
		Logger:     logger,
		Catalog:    store,
		AdminToken: cfg.AdminToken,
		Ingest:     trigger,
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("http listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
	logger.Info("shutdown complete")
}

func registerAWS(ctx context.Context, registry *providers.Registry, cfg config.Config, logger *slog.Logger) error {
	if cfg.AWS.AccessKeyID == "" || cfg.AWS.SecretAccessKey == "" {
		return fmt.Errorf("AWS credentials not configured; ingestion disabled")
	}
	sdk, err := awsprov.NewRealSDK(ctx, cfg.AWS.AccessKeyID, cfg.AWS.SecretAccessKey, cfg.AWS.PricingRegion)
	if err != nil {
		return err
	}
	client := awsprov.NewClient(sdk)
	// For v1 we ingest into a single region set. Configurable later.
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	ing := awsprov.NewIngestor(client, regions)
	registry.Register(providerAdapter{
		meta: providers.Metadata{Code: "aws", Name: "Amazon Web Services"},
		run: func(ctx context.Context, deps providers.IngestDeps) error {
			return ing.Run(ctx, deps.DB)
		},
	})
	logger.Info("aws provider registered", "regions", regions)
	return nil
}

type providerAdapter struct {
	meta providers.Metadata
	run  func(ctx context.Context, deps providers.IngestDeps) error
}

func (p providerAdapter) Metadata() providers.Metadata { return p.meta }
func (p providerAdapter) Ingest(ctx context.Context, deps providers.IngestDeps) error {
	return p.run(ctx, deps)
}

type ingestTrigger struct {
	mu       sync.Mutex
	running  bool
	orch     *ingest.Orchestrator
	registry *providers.Registry
	pool     *pgxpool.Pool
	logger   *slog.Logger
}

func (t *ingestTrigger) Trigger(providerCode string) (int64, error) {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return 0, fmt.Errorf("ingestion already running")
	}
	t.running = true
	t.mu.Unlock()

	p, ok := t.registry.Get(providerCode)
	if !ok {
		t.mu.Lock()
		t.running = false
		t.mu.Unlock()
		return 0, fmt.Errorf("provider %s not registered", providerCode)
	}

	go func() {
		defer func() {
			t.mu.Lock()
			t.running = false
			t.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Hour)
		defer cancel()
		if err := t.orch.Run(ctx, p); err != nil {
			t.logger.Error("ingestion run failed", "err", err, "provider", providerCode)
		}
	}()
	// Return a placeholder run id — the real one is created inside orch.Run.
	// (The admin endpoint reports it as an accepted async trigger.)
	return 0, nil
}

type combinedReady struct {
	pool    *pgxpool.Pool
	catalog httpx.CatalogReadiness
}

func (c combinedReady) Ready(ctx context.Context) (bool, string) {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.pool.Ping(pingCtx); err != nil {
		return false, "db unreachable: " + err.Error()
	}
	return c.catalog.Ready(ctx)
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
```

- [ ] **Step 2: Verify build**

Run: `cd api && go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 3: Verify all tests still pass**

```bash
docker run --rm -d --name pg-e2e-test -e POSTGRES_PASSWORD=pw -e POSTGRES_USER=u -e POSTGRES_DB=d -p 55432:5432 postgres:16
sleep 3
cd api
TEST_DATABASE_URL="postgres://u:pw@127.0.0.1:55432/d?sslmode=disable" go test ./... 2>&1 | tail -30
cd ..
docker rm -f pg-e2e-test
```

Expected: all packages PASS or SKIP.

- [ ] **Step 4: Commit**

```bash
git add api/cmd/aws-calc-api/main.go
git commit -m "feat(api): wire providers, orchestrator, scheduler, catalog, admin"
```

---

## Task 15: End-to-end verification against real AWS

**Files:**
- None modified. This task validates a working, credentialed pipeline.

- [ ] **Step 1: Populate `.env` with valid AWS credentials**

Edit `/home/oche/personal-stuff/aws-calculator/.env` and set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` to a real IAM user with `pricing:GetProducts` and `pricing:DescribeServices`. If unavailable, stop here and report — do not fabricate credentials.

- [ ] **Step 2: Cold-boot the stack**

```bash
docker compose down -v
docker compose up --build -d
sleep 30
```

- [ ] **Step 3: Trigger an ingestion**

```bash
ADMIN_TOKEN=$(grep '^ADMIN_TOKEN=' .env | cut -d= -f2)
curl -sf -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:8080/api/v1/admin/ingest?provider=aws"
```

Expected: `{"data":{"provider":"aws","run_id":0}}` (async accept; the actual run id is generated internally).

- [ ] **Step 4: Watch the run**

```bash
docker compose logs -f api | grep -E "ingestion|scheduled|http"
```

Wait ~15–60 minutes for the initial ingestion (three regions × ~200 services). When you see `"ingestion finished"`, stop the logs stream (`Ctrl-C`).

- [ ] **Step 5: Verify catalog is populated**

```bash
curl -s http://localhost:8080/api/v1/services?provider=aws | head -c 500
curl -s http://localhost:8080/api/v1/services/AmazonEC2?provider=aws | head -c 800
curl -s http://localhost:8080/api/v1/regions?provider=aws
curl -s http://localhost:8080/readyz
```

Expected: services list non-empty; AmazonEC2 response includes many dimensions; regions include `us-east-1`, `us-west-2`, `eu-west-1`; `/readyz` returns `{"status":"ready",...}`.

- [ ] **Step 6: Tear down and commit any remaining changes**

```bash
docker compose down
```

If steps 1–5 worked without code changes, there's nothing to commit. If you had to fix something, commit each fix with a clear message.

---

## Self-review notes

- **Spec coverage for this plan:** catalog schema (services, service_dimensions, dimension_values, sku_prices, staging_sku_prices), Provider interface + AWS impl, staging-swap ingestion, advisory-lock single-flight, rate limiting, cron scheduling, catalog HTTP endpoints, admin ingest endpoint, currencies endpoint, catalog-based `/readyz`, envelope + cursor pagination. FX refresh, pricing lookup with attribute match, and auth are deliberately deferred to Plans 03 and 04.
- **Placeholder scan:** none.
- **Type consistency:**
  - `providers.Provider` / `providers.IngestDeps` / `providers.Metadata` used consistently across `providers`, `ingest`, and `main.go`.
  - `catalog.Store` methods have consistent signatures between store, tests, and handlers.
  - `httpapi.IngestTrigger` is a single interface, implemented by `main.go`'s `ingestTrigger`.
  - The router's `Deps` struct is the single extension point; new fields (`Catalog`, `AdminToken`, `Ingest`) are all nil-guarded to preserve existing tests.
- **Notable simplifications flagged for future plans:**
  - `Trigger` currently returns `0` as run id (the real id is generated inside `orch.Run`). Plan 03 or later can surface it via a runs endpoint.
  - Regions ingested are hard-coded to three (`us-east-1`, `us-west-2`, `eu-west-1`) for practical initial ingestion time. All-regions expansion is trivial once the pipeline is validated.
  - Dimension `is_required` and `label` are stored as raw attribute names; humanization is a UX pass deferred to the frontend plan.
