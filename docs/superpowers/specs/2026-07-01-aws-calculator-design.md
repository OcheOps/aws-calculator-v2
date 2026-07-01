# AWS Calculator — Design Spec

**Date:** 2026-07-01
**Status:** Approved for planning
**Deployment target:** Contabo VPS (docker-compose)

## Goal

A web-based cloud cost calculator that uses live AWS Pricing API data to price arbitrary AWS resources across all services and regions, with per-user display currency, save/share estimates, and a schema designed to accept additional providers (GCP, Azure) later. Intended to potentially grow into a SaaS product.

## Non-goals (v1)

- Per-account negotiated discounts (EDP/PPA/CUR ingestion)
- Reading real AWS resource inventories (Cost Explorer, resource tagging)
- Reserved-instance / Savings-Plan optimization advice
- Providers other than AWS (schema supports them; implementation deferred)
- Team collaboration features beyond a single-owner workspace (invitations stubbed)
- Prometheus metrics endpoint (route reserved, wiring deferred)

## Choices summary

| Area | Decision |
|---|---|
| Data source | Live AWS Pricing API, list prices only |
| Coverage | Every service the Pricing API exposes, generic dimension renderer |
| Credentials | Server-side, single IAM user, users bring nothing |
| Cache | Postgres-backed catalog, daily scheduled refresh |
| Backend | Single Go service (HTTP API + ingestion goroutine + FX goroutine + mailer) |
| Frontend | SvelteKit (SSR, Node adapter) |
| Auth | Magic link (email → click → HttpOnly session cookie) |
| Persistence | Local anonymous save (edit_token) + authed save/share |
| Currency | User-picked; USD-base FX from exchangerate.host, refreshed hourly |
| Multi-cloud | `provider_id`-keyed schema; Provider interface in Go; AWS-only impl |
| DB | Postgres 16 |
| Deployment | docker-compose on Contabo (api, web, postgres, caddy) |

## Architecture

```
┌────────────────────┐        ┌──────────────────────────────────┐
│  SvelteKit (web)   │───────▶│  Go service (api + workers)      │
│  SSR + client      │  HTTPS │  - chi HTTP handlers             │
│  Session cookie    │        │  - Ingestion goroutine (cron)    │
└────────────────────┘        │  - FX refresh goroutine          │
                              │  - Magic-link mailer             │
                              └──────────────┬───────────────────┘
                                             │
                                    ┌────────▼─────────┐
                                    │   Postgres 16    │
                                    └──────────────────┘
                                             ▲
                              ┌──────────────┴──────────────┐
                              │ AWS Pricing API (us-east-1) │
                              │ FX API (exchangerate.host)  │
                              │ SMTP (magic links)          │
                              └─────────────────────────────┘
```

**Deployables** (docker-compose):
- `postgres` — Postgres 16 with named volume
- `api` — Go binary, port 8080 internal
- `web` — SvelteKit Node server, port 3000 internal
- `caddy` — TLS termination via Let's Encrypt, proxies `/api` → api, `/` → web

Secrets live in `.env` (git-ignored): SMTP creds, AWS access key/secret, session signing key, DB password, FX API key if needed.

### Provider abstraction

`internal/providers/Provider` interface with `Ingest(ctx) error` and `Metadata() ProviderMeta`. `providers/aws/` implements it. Adding GCP later is a new subpackage plus a row in `providers`. All downstream code (catalog, pricing, estimates, frontend) is provider-agnostic and keyed on `provider_id`.

## Data model

Postgres, one database, table-prefix namespacing.

### Provider & pricing catalog

```
providers            (id, code, name, enabled)
services             (id, provider_id, code, name, family,
                      description, icon_url)
service_dimensions   (id, service_id, key, label, kind,
                      value_type, is_required, sort_order)
dimension_values     (id, dimension_id, value, label)
sku_prices           (id, service_id, sku, region_code,
                      unit, currency, price_per_unit,
                      term_type, purchase_option,
                      lease_length, offering_class,
                      attributes JSONB,
                      effective_from, effective_to)
ingestion_runs       (id, provider_id, started_at, finished_at,
                      status, services_count, skus_count, error)
```

`dimension.kind` ∈ {`enum`, `region`, `numeric`, `freeform`}. `sku_prices.attributes` is JSONB with a GIN index; lookup uses `@>` containment for exact match, with fallback logic (see Data flows). Ingestion writes to a `staging_sku_prices` table and swaps into `sku_prices` inside a transaction so users never see a half-loaded catalog.

### Identity & workspaces

```
users                (id, email, email_verified_at, created_at, disabled_at)
magic_link_tokens    (id, user_id, token_hash, expires_at, used_at, ip, ua)
sessions             (id, user_id, token_hash, expires_at, last_seen_at)
organizations        (id, name, slug, plan, created_at)
memberships          (id, org_id, user_id, role)
api_keys             (id, org_id, name, key_hash, scopes,
                      created_at, last_used_at, revoked_at)
```

Tokens (magic link, session, API key, share) are stored as SHA-256 hashes — a DB leak does not grant sessions. `organizations` + `memberships` model exists from day one to support multi-user SaaS later; v1 auto-creates a single default org per user (`"<email-local>'s workspace"`) and does not expose invitation UI. `api_keys` is schema-only in v1 (no auth enforcement path).

### Estimates

```
estimates            (id, org_id, created_by, name, currency,
                      fx_rate_snapshot, notes, created_at, updated_at)
estimate_items       (id, estimate_id, service_id, region_code,
                      sku_id, quantity, unit_override,
                      attributes_snapshot JSONB,
                      unit_price_snapshot, currency_snapshot,
                      monthly_cost_snapshot)
estimate_shares      (id, estimate_id, share_token, expires_at,
                      permission, created_at)
```

`org_id` is nullable so anonymous estimates work. Anonymous estimates carry an `edit_token` (returned to the client, stored in localStorage). On sign-up, the client passes any known `edit_token`s and the API claims them into the new user's default org.

Items snapshot unit price and attributes at add-time. Repricing is an explicit `POST /estimates/:id/reprice` that returns a diff for user confirmation before committing.

### FX & audit

```
fx_rates             (base, quote, rate, fetched_at)   -- base always USD
audit_log            (id, org_id, user_id, action, entity,
                      entity_id, meta JSONB, created_at)
```

Migrations via `golang-migrate`, SQL files, up/down, versioned in `migrations/`.

## Data flows

### Ingestion (goroutine, daily 03:00 UTC)

Per enabled provider, `Provider.Ingest(ctx)` runs. AWS impl:

1. `pricing:ListServices` → upsert `services`.
2. `pricing:DescribeServices` per service → derive attribute keys → upsert `service_dimensions`.
3. Per region in the service's region list: paginate `pricing:GetProducts` with a region filter. Parse each product's SKUs and price dimensions. Write into `staging_sku_prices`. Collect distinct enum values → upsert `dimension_values`.
4. Checkpoint progress into `ingestion_runs` per service.
5. On success, atomically swap `staging_sku_prices` into `sku_prices` within a transaction.

**Reliability:** single-flight via `sync.Mutex` plus a Postgres advisory lock (survives restarts). Token-bucket rate limiter at 8 rps (below AWS's ~10 rps limit). Exponential backoff on `ThrottlingException`. Crashes resume at the next un-checkpointed service.

**Bootstrap:** `/readyz` returns 503 until `ingestion_runs` has at least one row with `status='success'`. The web UI surfaces a "Loading pricing catalog" state.

### FX refresh (goroutine, hourly)

Fetch USD-base rates from `exchangerate.host` (no API key required). Upsert into `fx_rates`. Failures do not block pricing lookups — the last known rate is used and the failure is logged with the age of the last successful rate.

### Pricing lookup (hot path)

```
GET /api/v1/price?service=AmazonEC2&region=us-east-1
    &attr[instanceType]=m5.large&attr[operatingSystem]=Linux
    &attr[tenancy]=Shared&attr[preInstalledSw]=NA
    &term=OnDemand&currency=NGN
```

1. Validate: service exists, region exists, all `is_required` dimensions present.
2. Query `sku_prices` with JSONB `@>` on the requested attributes plus region + term_type.
3. Exact hit → return price + `matchQuality: "exact"`.
4. No hit → drop optional attributes one at a time in `sort_order` and retry. Return `matchQuality: "partial"` with the list of attributes dropped.
5. Convert USD price to requested currency via `fx_rates` (latest quote). Response includes `currency`, `fxRate`, `fxAsOf`.
6. Cache in-process (Ristretto, 5-minute TTL) keyed on the full canonical query.

### Magic-link auth

```
POST /api/v1/auth/request-link { email }
```

1. Rate-limit: 5/hour per IP AND per email address.
2. Lowercase-normalize email. Upsert user record.
3. Generate 32 random bytes, store `SHA-256(token)` in `magic_link_tokens` with 15-minute TTL and single-use flag.
4. Send email via SMTP with link `https://<host>/auth/verify?token=<raw>`.
5. Always return HTTP 202 regardless of whether the email exists (no enumeration).

```
POST /api/v1/auth/verify { token }
```

1. Look up by `SHA-256(token)`. Reject if `used_at IS NOT NULL` or expired.
2. Mark `used_at`, set `users.email_verified_at`.
3. Create session (32 bytes, hashed, 30-day sliding TTL). Set cookie `session=<raw>; HttpOnly; Secure; SameSite=Lax; Path=/`.
4. If user has no memberships, create a default org `"<email-local>'s workspace"` and add owner membership.
5. Accept optional `claim_tokens[]` in body; claim any anonymous estimates matching them into the default org.
6. Return the user profile.

### Save / share estimate

Anonymous:
- `POST /api/v1/estimates` with `org_id=NULL`, `created_by=NULL`. Returns `{ id, edit_token, share_token }`. Client stores `edit_token` in localStorage.

Authenticated:
- `POST /api/v1/estimates` scoped to active org, standard CRUD.
- `POST /api/v1/estimates/:id/share` → returns share URL.

Public share view:
- `GET /s/:share_token` — read-only render; if the viewer is logged in, offer "Clone to my workspace".

## HTTP API surface

Router: chi. Envelope: `{ data, meta, error }`. Errors: RFC 7807. Every response carries `X-Request-Id`. Structured logs via `slog` in JSON.

```
Catalog
  GET  /api/v1/providers
  GET  /api/v1/services?provider=aws&q=&cursor=&limit=
  GET  /api/v1/services/:id
  GET  /api/v1/services/:id/dimension-values?dim=&region=
  GET  /api/v1/regions?provider=aws
  GET  /api/v1/currencies
  GET  /api/v1/price?…

Auth
  POST /api/v1/auth/request-link
  POST /api/v1/auth/verify
  POST /api/v1/auth/logout
  GET  /api/v1/auth/me

Orgs (scaffolding)
  GET  /api/v1/orgs
  POST /api/v1/orgs
  GET  /api/v1/orgs/:slug/members
  POST /api/v1/orgs/:slug/invitations       # stub — returns 501 in v1

Estimates
  POST   /api/v1/estimates
  GET    /api/v1/estimates
  GET    /api/v1/estimates/:id
  PATCH  /api/v1/estimates/:id
  DELETE /api/v1/estimates/:id
  POST   /api/v1/estimates/:id/items
  PATCH  /api/v1/estimates/:id/items/:item_id
  DELETE /api/v1/estimates/:id/items/:item_id
  POST   /api/v1/estimates/:id/reprice
  POST   /api/v1/estimates/:id/share
  GET    /api/v1/estimates/:id/export?format=pdf|csv|json

Public
  GET  /api/v1/shared/:share_token

Ops
  GET /healthz
  GET /readyz
```

Pagination is cursor-based (`?cursor=&limit=`, opaque base64 cursor).

OpenAPI 3.1 spec lives in `api/openapi.yaml` and is the source of truth for both server and typed client generation.

## Go project layout

```
cmd/aws-calc-api/main.go
internal/
  config/        env → struct
  db/            pgx pool, migration bootstrap
  http/          chi router, middleware (auth, ratelimit, request-id, recoverer, CORS)
  auth/          magic-link + session logic
  catalog/       services, dimensions, regions
  pricing/       lookup + cache
  estimates/     CRUD, snapshots, reprice, export (PDF via gofpdf, CSV, JSON)
  fx/            rate refresh + convert
  mail/          SMTP sender (interface, stub for tests)
  providers/     Provider interface
    aws/         aws-sdk-go-v2 pricing ingestor
  workers/       cron scheduler wiring (robfig/cron/v3)
  audit/         write-only helper
migrations/*.sql
api/openapi.yaml
```

## SvelteKit structure

```
src/
  routes/
    +layout.svelte
    +page.svelte                      # dashboard
    services/+page.svelte             # catalog browse
    services/[code]/+page.svelte      # DimensionForm host
    estimates/[id]/+page.svelte       # editor
    s/[token]/+page.svelte            # read-only shared view
    auth/request/+page.svelte
    auth/verify/+page.server.ts       # forwards token to API, sets cookie
  lib/
    api/                              # generated typed client from openapi.yaml
    stores/                           # session, active org, currency, cart
    components/
      DimensionForm.svelte
      PriceBadge.svelte
      EstimateTable.svelte
      MatchQualityBadge.svelte
```

`DimensionForm` is the core UX primitive: given a service's `dimensions[]`, it renders inputs by kind (enum → select with fetched values, region → region picker, numeric → number input, freeform → text), reveals dependent dimensions only after prerequisites are chosen (`instanceType` values are fetched only once `region` is set), and debounces `/price` calls for live estimation.

## Testing

**Go**
- Unit: pricing attribute-match ranking, FX conversion math, token hashing, snapshot freezing. Table-driven.
- Integration: `testcontainers-go` Postgres, run migrations, seed fixture SKUs, drive HTTP handlers via `httptest`. Auth flow end-to-end with a fake mailer that captures sent links.
- Ingestor: AWS Pricing client hidden behind an interface; unit tests use recorded JSON fixtures for EC2 m5.large, S3 Standard, Lambda. Verifies parse → upsert → dimension extraction.
- Contract: an OpenAPI conformance test walks every route and checks the handler matches the spec.

**Frontend**
- Unit: `vitest` for stores and `DimensionForm` state machine.
- Component: Svelte Testing Library for `DimensionForm`, `EstimateTable`.
- E2E: Playwright, one happy path per major flow — browse → add item → save anon → sign up → claim → share → open share in a fresh context.

**Explicitly out of scope for CI**: live AWS Pricing API calls (flaky, slow, requires secrets). A `make ingest-smoke` target hits real AWS against one service in one region for pre-release manual verification.

**CI**: GitHub Actions runs `go test ./...`, `svelte-check`, `vitest`, and Playwright against `docker compose up` on every PR.

## Deployment

`docker-compose.yml` at repo root:

- `postgres`: `postgres:16`, named volume `pgdata`, healthcheck.
- `api`: built from `./api`, `depends_on: [postgres]`, env from `.env`.
- `web`: built from `./web`, `depends_on: [api]`, env from `.env`.
- `caddy`: `caddy:2`, ports `80` and `443`, mounts `Caddyfile` and `caddydata` volume. Automatic Let's Encrypt.

`docker-compose.override.yml` provides local dev with source mounts and hot reload (air for Go, `pnpm dev` for SvelteKit).

Secrets sourced from `.env` (git-ignored): `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `SMTP_HOST/PORT/USER/PASS`, `SESSION_SIGNING_KEY`, `POSTGRES_PASSWORD`, `PUBLIC_BASE_URL`.

## Security notes

- All secret tokens (magic link, session, share, api_key) stored as SHA-256 hashes.
- Cookies: `HttpOnly; Secure; SameSite=Lax`. No credentials in localStorage.
- CSRF: SameSite=Lax plus double-submit token on state-changing endpoints from browser clients.
- Rate limits (in-memory token buckets per key):
  - Magic link request: 5/h per IP AND per email.
  - Auth verify: 30/h per IP.
  - Anonymous estimate create: 20/h per IP.
- Email enumeration avoided: request-link always returns 202.
- No admin panel in v1. DB operations happen via `psql`.

## Resolved decisions

- **SMTP for magic links**: Brevo (formerly Sendinblue) via SMTP relay. Credentials in `.env` as `SMTP_HOST=smtp-relay.brevo.com`, `SMTP_PORT=587`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`.
- **Default display currencies**: USD, EUR, GBP, NGN. User can pick per-estimate; USD is the underlying storage currency.
- **Admin ingest route**: `POST /api/v1/admin/ingest` protected by a static bearer token in `.env` (`ADMIN_TOKEN`). Triggers an out-of-band ingestion run for the given provider. Returns 202 with a run ID.
- **PDF/CSV export layout**:
  - PDF: cover page (estimate name, org, generated-at, currency, FX-as-of), summary table (service, region, monthly cost), itemized table per service (attributes snapshot rendered as key: value), totals footer. Generated with `gofpdf`.
  - CSV: flat rows — `service, region, sku, quantity, unit, unit_price, monthly_cost, currency, attributes_json`.
  - JSON: full estimate envelope including snapshots (schema-versioned as `"version": 1`).

## UI quality bar

The frontend is a first-class part of the product, not a debug shell over the API. Concretely:

- Design system: Tailwind + a small set of custom Svelte primitives (Button, Input, Select, Combobox, Card, Sheet, Toast). No off-the-shelf component library that fights our aesthetic.
- Type: Inter for UI, JetBrains Mono for prices/SKUs. Tabular figures for all money.
- Layout: two-pane on desktop (catalog/form left, live-updating estimate cart right), single-pane on mobile with a sticky total.
- Motion: subtle — 150ms ease transitions on state changes, no bounce, no gratuitous scroll animation.
- Empty and loading states are designed, not stubbed. Skeletons match final layouts.
- Accessible: keyboard-navigable throughout, labeled inputs, visible focus rings, contrast ≥ WCAG AA.
- Dark mode from day one, driven by system preference with manual override.
- Reference the `frontend-design` skill during implementation to keep the aesthetic distinctive, not generic.
