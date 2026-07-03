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
    effective_to      TIMESTAMPTZ
);
CREATE UNIQUE INDEX sku_prices_unique
    ON sku_prices (service_id, sku, region_code, term_type, COALESCE(purchase_option,''), COALESCE(lease_length,''));
CREATE INDEX sku_prices_service_region ON sku_prices (service_id, region_code);
CREATE INDEX sku_prices_attrs_gin ON sku_prices USING GIN (attributes jsonb_path_ops);

CREATE TABLE staging_sku_prices (LIKE sku_prices INCLUDING ALL);
