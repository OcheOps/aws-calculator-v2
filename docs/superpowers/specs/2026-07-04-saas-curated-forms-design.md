# AWS Calculator — SaaS Curated-Forms Redirect

**Date:** 2026-07-04
**Status:** Approved for planning
**Supersedes:** the frontend portion of `2026-07-01-aws-calculator-design.md` (Plan 06). Backend from Plans 01–08 is unchanged.

## Why this redirect exists

The original Plan 06 shipped a *generic dimension renderer*: for every AWS service, dump the raw AWS Pricing API attribute list into 15–82 dropdowns labelled with technical AWS keys (`preInstalledSw`, `capacitystatus`, `dedicatedEbsThroughputDescription`…). It scales to all 268 services for free, but it produces a page nobody wants to use. On EC2 the user faces ~80 attribute keys most humans never think about.

An AWS calculator that "wins" over AWS's own calculator, Vantage, Infracost, and Pricelist.io has to feel *use-case first*, not attribute-first. Pick a service, answer 3–6 human questions ("how many m5.larges, what OS, how many hours a month"), get a price, add to an estimate.

This spec rewrites the frontend to that model, keeps the entire backend intact, and preserves the generic renderer behind an `Advanced → All services` toggle so the long tail of ~250 services still works.

## Positioning

**Product.** An AWS pricing calculator with three wedges over AWS's own calculator and the existing paid alternatives:

1. Cleaner UX for the common case (curated forms per service, ≤6 inputs each).
2. First-class multi-currency (USD, EUR, GBP, NGN, INR, ZAR, KES, JPY, …), FX-refreshed hourly, snapshotted on the estimate at save time.
3. Shareable estimates as first-class artefacts: read-only web link + branded PDF export.

**Explicitly not a wedge in v1.** Reading a real AWS account (Cost Explorer / role-assume). That's a different product; deferred.

**Monetization.** None in v1. Fully free. Audience first, monetize later based on demand signal.

**Audience.** Public SaaS, general market. UX bar is set accordingly: no jargon in the primary flow, mobile-usable, share URLs meaningful when opened by non-AWS people.

## v1 curated services (12)

Ordered by rollout priority; all ship in the same window but EC2 pilots the pattern.

| Service | Human inputs |
|---|---|
| **EC2** | Instance type · OS · Tenancy · Count · Usage preset (Full month / Weekday hours / Custom) |
| **S3** | Storage class · Storage GB · PUT + GET requests / month |
| **EBS** | Volume type · Size GB · IOPS (for io2) · Throughput MB/s (for gp3) |
| **Lambda** | Memory MB · Duration ms · Invocations / month |
| **CloudFront** | Data-out region · Data-out GB · HTTPS requests / month |
| **ALB** | LCU-hours (avg concurrent) · Data processed GB · Region |
| **Data Transfer** | Source · Destination (Internet / Same region / Inter-region) · GB / month |
| **RDS** | Engine · Instance size · Storage GB · Single-AZ vs Multi-AZ · Backup GB |
| **RDS Aurora** | Engine · Instance size · Storage GB · IO requests / month |
| **ElastiCache** | Engine · Node size · Node count |
| **DynamoDB** | Mode (on-demand / provisioned) · Storage GB · Read + write units |
| **Route53** | Hosted zones · Queries / month |

The remaining ~250 services stay pricable via the existing dimension renderer at `/services/[code]`, reachable from the side-sheet's `Advanced: browse all 268 services →` link.

## Interaction model

**Estimate-first.** No cart. The user always operates against a real, server-side estimate. On the landing page, the moment the user touches anything, the pre-baked example is implicitly forked into a new anonymous estimate and the URL becomes `/estimates/<id>?edit=<token>`.

**Side-sheet for adding.** Clicking `Add resource` in the estimate editor opens a right-hand side-sheet (420 px wide, full-height on mobile). Two states:

- **Picker state**: search box at top, 12 curated service tiles below (icon + name + one-liner), and an `Advanced: browse all 268 services →` link that navigates to `/services?to=<id>&edit=<token>`. Search matches curated first, then long-tail service codes.
- **Form state**: the picked service's curated form. Header shows `Add EC2` with a back arrow to the picker. Footer shows a live-updating cost preview (`≈ $54.20 / month`) and `Cancel` / `Add to estimate` buttons.

Editing an existing line opens the sheet directly in form state with fields pre-populated from `user_input_snapshot`. Button label switches to `Save changes`.

## Routes

| Route | Behaviour |
|---|---|
| `/` | Hero + pitch + display-currency picker (defaults to browser locale). Below the fold, a live-editable example estimate rendered exactly like `/estimates/[id]` with `forkOnEdit=true`. |
| `/estimates/[id]` | Estimate editor. Two-column on desktop (main table + right sidebar with name/currency/notes/share/export), single-column with collapsed sidebar on mobile. Anonymous or authed. |
| `/estimates/[id]?edit=<token>` | Same page, authorised via `X-Edit-Token` for anonymous ownership. |
| `/estimates` | Authed-only: list of saved estimates in the user's default org. |
| `/s/[token]` | Public read-only shared view. Same layout, no editing controls, small "Made with AWSCalculator" footer. |
| `/services` | **Advanced mode**. Existing 268-service grid + dimension renderer, unchanged internally. When arrived at with `?to=<id>&edit=<token>`, "Add to estimate" POSTs to that estimate and redirects back. |
| `/services/[code]` | Existing generic dimension form. Same `?to=&edit=` behaviour. |
| `/auth/request`, `/auth/verify` | Magic-link flow. No change from Plan 04. |

## Estimate editor layout

Two-column on desktop (`max-width: 1200px`; main column is `1fr`, sidebar is `320px`):

- **Main column** — a table grouped by service:
  - Group header row: service icon + name + collapse chevron.
  - Item rows: plain-English summary (`2 × m5.large in US East (N. Virginia) · Linux, 730 h/mo`), monthly cost in display currency, edit + delete buttons.
  - Group subtotal row: `Subtotal — EC2`, sum for the group.
  - Grand total row: sticky at the bottom of the main column.

  A single `Add resource` button lives above the table.

- **Right sidebar** — estimate name (inline-edit), display-currency picker, notes (textarea), `Share` button, `Export ▾` (PDF / CSV / JSON). On mobile these collapse into a top header + a floating action button for `Add resource`.

The share view (`/s/[token]`) renders the same table with all interactive controls hidden; the sidebar shrinks to just the summary + a "Clone to my workspace" button visible only to signed-in viewers.

## Curated-form abstraction

**File layout.**

```
web/src/lib/services/
  index.ts                          # curated registry
  ec2/
    Form.svelte                     # ~100 lines, uses shared primitives
    mapper.ts                       # userInput → PricedComponent[]
    mapper.test.ts                  # vitest unit tests
    icon.svg
  s3/ …
  rds/ …
  … 10 more
```

**Shared primitives** in `web/src/lib/components/`:

- `Combobox.svelte` — searchable, keyboard-navigable. For instance types, engines, etc.
- `PresetSelect.svelte` — radio-group with a "Custom…" escape hatch. Used for hours-per-month presets.
- `NumberInput.svelte` — number field with an inline unit label (`GB`, `reqs/mo`, `TB out`).
- `RegionPicker.svelte` — grouped by continent, defaults to `us-east-1`.

**Mapper contract.** Each service exports:

```ts
export const meta = {
  code: 'ec2',
  label: 'EC2 (compute)',
  description: 'Virtual servers by the hour.',
  icon: '/icons/ec2.svg',
  category: 'Compute'
};

export type EC2Input = { instanceType: string; operatingSystem: string; tenancy: string; count: number; hoursPerMonth: number };
export const defaults: EC2Input = { instanceType: 'm5.large', operatingSystem: 'Linux', tenancy: 'Shared', count: 1, hoursPerMonth: 730 };

export function map(input: EC2Input, region: string): PricedComponent[] {
  return [{
    service: 'AmazonEC2',
    attributes: {
      instanceType: input.instanceType,
      operatingSystem: input.operatingSystem,
      tenancy: input.tenancy,
      preInstalledSw: 'NA',
      capacitystatus: 'Used'
    },
    quantity: input.count * input.hoursPerMonth,
    unitLabel: 'hours',
    summary: `${input.count} × ${input.instanceType} in {region}, ${input.operatingSystem}, ${input.hoursPerMonth} h/mo`
  }];
}
```

`PricedComponent[]` lets composite services (RDS = instance + storage + backup, Lambda = invocations + duration, CloudFront = data-out + requests) express themselves as one row in the estimate with multiple underlying `/price` lookups. Simple services return a single-element array.

The registry (`services/index.ts`) is a `readonly Record<string, { meta, defaults, map, Form }>` used by the side-sheet picker to render tiles and dispatch to the right form.

**Data flow inside the sheet.**

```
[Add resource]
  → sheet state = 'picker'
  → user picks EC2
  → state = 'form:ec2'; lazy import('$lib/services/ec2/Form.svelte')
  → form state = ec2.defaults
  → user edits → debounce(200ms):
      components = ec2.map(input, region)
      results = await Promise.all(components.map(c =>
        api.price({ service: c.service, region, currency, attributes: c.attributes })
      ))
      footer.total = Σ (r.price_per_unit × c.quantity)
  → [Add to estimate]:
      POST /estimates/:id/items {
        line_type: 'curated',
        service_key: 'ec2',
        region_code: region,
        user_input_snapshot: input,
        components_snapshot: results.map((r,i) => ({
          sku: r.sku, unit: r.unit,
          unit_price: r.price_per_unit,
          quantity: components[i].quantity,
          monthly_cost: r.price_per_unit * components[i].quantity
        })),
        monthly_cost_snapshot: Σ components.monthly_cost,
        currency_snapshot: currency.value
      }
```

## Backend changes

**Migration** — single, additive, safe on live data.

```sql
-- 0006_curated_lines.up.sql
ALTER TABLE estimate_items
  ADD COLUMN line_type          TEXT NOT NULL DEFAULT 'legacy',
  ADD COLUMN service_key        TEXT,
  ADD COLUMN user_input_snapshot JSONB,
  ADD COLUMN components_snapshot JSONB;

CREATE INDEX estimate_items_service_key ON estimate_items (service_key)
  WHERE service_key IS NOT NULL;
```

- Existing rows are `line_type='legacy'`. Their `attributes_snapshot`, `unit_price_snapshot`, `monthly_cost_snapshot` remain the source of truth.
- Curated rows set `service_key`, `user_input_snapshot`, and `components_snapshot`. Their `monthly_cost_snapshot` is the sum of `components_snapshot[].monthly_cost`; the top-level `sku` becomes a synthetic marker like `curated:ec2:v1`.

**Handlers.**

- `POST /estimates/:id/items` accepts either shape. Validates `line_type`, requires `components_snapshot` when curated.
- `PATCH /estimates/:id/items/:itemId` handles both shapes.
- `POST /estimates/:id/reprice` walks each row; for both legacy and curated, re-hits `/price` with the stored per-component attributes and returns a `PriceDelta` array unchanged in shape. Curated rows produce N sub-deltas summed into a single `item_id` entry.
- `GET /estimates/:id/export?format=…` renders human summaries. For curated rows, use `service_key` + `user_input_snapshot` to produce the summary (via a small server-side mapping table of `service_key → summary template`), or fall back to reading the first component's SKU + attributes.

**Handler tests** cover: legacy round-trip (unchanged behaviour), curated round-trip preserves `components_snapshot`, reprice loops over components correctly, export walks both.

## Currency change

Estimate has a `currency` column already (Plan 05). Changing it in the sidebar:

1. Client `PATCH /estimates/:id { currency: 'NGN' }`.
2. Client iterates over items, for each: re-`/price` at the new currency, `PATCH` the row's `unit_price_snapshot`, `monthly_cost_snapshot`, `currency_snapshot` (and `components_snapshot` entries for curated).
3. The estimate stores a snapshot of the FX rate applied (`fx_rate_snapshot` column, already present) so the PDF footer can print "USD → NGN at 1370.99 as of 2026-07-04".

Persisting converted values (rather than converting on read) keeps saved totals stable when FX moves. This matches user expectation for a pricing tool.

## What gets removed

- `web/src/lib/stores/cart.svelte.ts` — no local cart; everything is server-side estimate.
- `web/src/lib/components/EstimateCart.svelte` — superseded by the estimate editor itself.
- The `Services` link in the top navigation — Advanced mode is reachable only from the side-sheet picker.

## What stays intact from Plans 01–08

- All backend packages: `catalog`, `pricing`, `fx`, `auth`, `estimates`, `export`, `providers/aws` (bulk ingestor), `ingest/pricefiles`, `httpapi`, `http`.
- Bulk ingest via price-list files (Plan 07). 30-minute first ingest, ETag-skipped incrementals in seconds.
- FX from `open.er-api.com`.
- Magic-link auth with Gmail SMTP.
- Share tokens, PDF/CSV/JSON export.
- Dimension renderer at `/services/[code]` — moved behind Advanced, otherwise unchanged.

## Rollout

Ten independent steps, `main` deployable throughout.

1. Migration `0006_curated_lines.up.sql` + handler tweaks. Additive only.
2. Shared primitives (`Combobox`, `PresetSelect`, `NumberInput`, `RegionPicker`) with unit tests. Not wired.
3. **EC2 pilot** — mapper + form + tests, added to the registry alone.
4. New estimate editor at `/estimates/[id]` — grouped table, sticky total, sidebar. Reads legacy and curated. `Add resource` opens the side-sheet with EC2 available + Advanced link.
5. New landing at `/` — hero + live example estimate + fork-on-edit.
6. Advanced-mode context (`/services?to=&edit=`) so the existing dimension form can add to the active estimate.
7. Delete cart store, EstimateCart, top-nav Services link.
8. Roll out the remaining 11 curated services in one batch. Each is a ~200-line PR.
9. Currency-change per-line PATCH sequence.
10. Playwright happy-path e2e covering land → edit example → fork → sign up → claim → share → export.

Between steps 3 and 8 the app is fully usable: EC2 via the new UX, everything else via Advanced.

## Testing

- **Mappers** — `vitest` unit tests, one `mapper.test.ts` per service. High leverage: if the mapper is right, the form is right. Assertions: `defaults` produces valid input; `map(defaults, 'us-east-1')` returns the expected component shape and quantity; edge cases (e.g. RDS Multi-AZ doubles instance quantity) produce correct components.
- **Backend Go tests** — extend existing `internal/estimates` and `internal/httpapi/estimates_test.go`: curated line create/get round-trip preserves `components_snapshot`; reprice loops sum correctly; export walks curated lines.
- **Component tests** — Svelte Testing Library on the four shared primitives only (Combobox keyboard-nav, PresetSelect toggling, NumberInput unit label, RegionPicker grouping). Per-service forms are thin enough that e2e coverage is more valuable than component tests.
- **Playwright happy path** — one long test:
  1. Land on `/`, change EC2 quantity in the example to 3.
  2. Confirm redirect to `/estimates/<id>?edit=<token>`.
  3. Open `Add resource`, add an S3 line.
  4. Change display currency to NGN; assert line totals non-zero.
  5. Request magic link, verify via FakeMailer, confirm estimate claim.
  6. Share, open share URL in a fresh context.
  7. Export PDF, assert content-type + non-empty response.
- **Regression** — a fixture that seeds a legacy line in `estimate_items` before running the app; assert it renders in the new editor and reprices without error.

## Explicitly out of scope (v1)

- Branded / custom PDFs beyond a static "Made with AWSCalculator" footer.
- Team invitations UI.
- AWS account read (Cost Explorer / role-assume).
- Per-line comments on shared estimates.
- Workload templates ("Small web app", "ML training rig"). Straightforward follow-up; deferred.
- Removing legacy line support from the backend (kept indefinitely).
