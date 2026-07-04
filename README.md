# AWS Calculator

A cloud cost calculator that prices AWS resources against live data from the AWS Pricing API, in the currency you actually think in. Ships as a SvelteKit web UI over a Go API, backed by Postgres.

## What it does

- **Live AWS prices.** ~530,000 SKUs across 268 services, refreshed nightly from AWS's bulk price-list files. First ingest takes ~30 minutes; subsequent runs finish in seconds via ETag-skip.
- **Multi-currency.** USD is the source of truth; FX rates from `open.er-api.com` refresh hourly. Display prices in NGN, EUR, GBP, and others; each estimate snapshots the FX rate applied.
- **Estimate-first UX (in progress).** Land in a live example estimate, edit inline, click *Add resource* to open a right-hand side-sheet with a hand-crafted per-service form. First curated service (EC2) is live; ten more follow in Plan 11.
- **Advanced mode.** Every service in the catalog is still reachable via a generic dimension renderer at `/services` — behind an *Advanced* link, not the front door.
- **Save + share + export.** Anonymous estimates get a private edit token; sign-in via magic link claims them into your workspace. Share via a public read-only link; export as PDF, CSV, or JSON.
- **Magic-link auth.** Passwordless via Gmail SMTP (or Brevo, or any SMTP host). Session cookies are `HttpOnly; SameSite=Lax`. Tokens are stored as SHA-256 hashes.

## Architecture

```
┌────────────────────┐        ┌──────────────────────────────────┐
│  SvelteKit (web)   │───────▶│  Go service (api + workers)      │
│  SSR + client      │  HTTPS │  chi HTTP + ingest goroutines    │
└────────────────────┘        └──────────────┬───────────────────┘
                                             ▼
                                    ┌──────────────────┐
                                    │   Postgres 16    │
                                    └──────────────────┘
                                             ▲
                              ┌──────────────┴──────────────┐
                              │ AWS Pricing (us-east-1)     │
                              │ open.er-api.com (FX)        │
                              │ SMTP (magic links)          │
                              └─────────────────────────────┘
```

Everything runs in docker-compose on a single host — Postgres, the Go API, the SvelteKit web server, and Caddy for TLS termination in production.

## Quick start

```bash
cp .env.example .env
# fill in AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY (pricing:* read-only IAM user)
# optionally set SMTP_HOST/USER/PASS/FROM for real magic-link emails
docker compose up --build -d
```

Then open <http://localhost:3000>. If SMTP isn't configured, magic-link URLs print to `docker compose logs api`.

### Trigger the first ingest

The catalog is empty until you run one:

```bash
curl -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://localhost:8080/api/v1/admin/ingest?provider=aws
```

Progress lives in the logs. Subsequent runs are ETag-diffed and complete in seconds unless AWS's price files actually changed.

## Repository layout

```
api/                       # Go 1.25 API + ingest worker
  cmd/aws-calc-api/        # main.go
  internal/
    catalog/               # services + dimensions + regions store
    pricing/               # /price lookup + Ristretto cache + FX conversion
    fx/                    # currency refresh + store
    auth/                  # magic-link tokens + sessions + orgs
    estimates/             # CRUD + reprice + curated vs legacy line handling
    export/                # PDF / CSV / JSON
    httpapi/               # HTTP handlers (envelope, catalog, price, auth, estimates)
    ingest/                # orchestrator + advisory-lock single-flight
      pricefiles/          # ETag store for skip-unchanged
    providers/
      aws/                 # bulk ingestor (price-list files) + client wrapper
    workers/               # cron scheduler
  migrations/              # golang-migrate SQL
web/                       # SvelteKit 2 + Svelte 5 (runes) + Tailwind 4
  src/lib/
    api/                   # typed client + DTOs (mirror Go)
    components/            # design system primitives + estimate widgets
    services/              # curated per-service mappers + Form.svelte
      ec2/                 # first pilot service (Plan 09)
    utils/                 # labels, format, summary, example-estimate
    stores/                # theme, session, currency (Svelte 5 runes)
  src/routes/              # SvelteKit pages
docs/superpowers/
  specs/                   # design docs (product decisions)
  plans/                   # step-by-step implementation plans
```

## Environment variables

Everything is in `.env.example`. The important ones:

| Var | Purpose |
|---|---|
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | Server-side pricing:* read-only IAM user. |
| `AWS_PRICING_REGION` | Must be `us-east-1` or `ap-south-1` (AWS Pricing endpoints). |
| `ADMIN_TOKEN` | Bearer for `POST /api/v1/admin/ingest`. |
| `INGEST_CRON` | Nightly refresh (default `0 3 * * *`). |
| `INGEST_REGION_CONCURRENCY` | Parallel per-region fetches (default 3). |
| `INGEST_BATCH_SIZE` | Rows per `COPY` into staging (default 5000). |
| `FX_API_URL` | `https://open.er-api.com` — free, no key. |
| `FX_CRON` | Rate refresh cadence (`@hourly`). |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM` | Magic-link mailer. Leave user/pass empty to fall back to a logging FakeMailer. |
| `PUBLIC_BASE_URL` | Used in magic-link URLs. |

## Development

```bash
# Backend
cd api
go test ./...                       # unit + integration (integration tests need TEST_DATABASE_URL)
go build ./...

# Frontend
cd web
npm install
npm run check                       # svelte-check
npm run test                        # vitest (mappers + primitives)
npm run build                       # production build (Node adapter)
npm run dev                         # dev server on :3000
```

Integration tests spin up ephemeral Postgres containers themselves.

## Design docs

Read these before touching the design:

- `docs/superpowers/specs/2026-07-01-aws-calculator-design.md` — original spec (backend architecture, catalog schema, plans 01–08).
- `docs/superpowers/specs/2026-07-04-saas-curated-forms-design.md` — v2 spec (SaaS positioning, estimate-first UX, curated per-service forms).

Implementation plans in `docs/superpowers/plans/` are numbered by shipping order.

## Status

| Plan | Scope | Status |
|---|---|---|
| 01 | Foundation (Go/SvelteKit/Postgres/Caddy/docker-compose) | Shipped |
| 02 | AWS ingestor + catalog API | Shipped |
| 03 | Pricing lookup + FX | Shipped |
| 04 | Magic-link auth + orgs | Shipped |
| 05 | Estimates + share + export | Shipped |
| 06 | SvelteKit UI (initial generic dimension renderer) | Shipped |
| 07 | Bulk price-list ingest + ETag-skip incremental | Shipped |
| 08 | Retire the paged ingestor | Shipped |
| 09 | Curated-forms foundation + EC2 pilot | Shipped |
| 10 | New editor, landing, Advanced-mode, cleanup | In progress |
| 11 | Remaining 11 curated services + currency change + e2e | Pending |

## License

Not yet chosen — assume all rights reserved for now.
