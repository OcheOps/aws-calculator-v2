CREATE TABLE estimates (
    id                BIGSERIAL PRIMARY KEY,
    org_id            BIGINT REFERENCES organizations(id) ON DELETE CASCADE,
    created_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    name              TEXT NOT NULL,
    currency          TEXT NOT NULL DEFAULT 'USD',
    fx_rate_snapshot  JSONB NOT NULL DEFAULT '{}'::jsonb,
    notes             TEXT,
    edit_token_hash   TEXT,
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
    id               BIGSERIAL PRIMARY KEY,
    estimate_id      BIGINT NOT NULL REFERENCES estimates(id) ON DELETE CASCADE,
    share_token_hash TEXT NOT NULL UNIQUE,
    expires_at       TIMESTAMPTZ,
    permission       TEXT NOT NULL DEFAULT 'read',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
