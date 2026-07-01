CREATE TABLE providers (
    id          BIGSERIAL PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO providers (code, name) VALUES ('aws', 'Amazon Web Services');

CREATE TABLE fx_rates (
    id          BIGSERIAL PRIMARY KEY,
    base        TEXT NOT NULL,
    quote       TEXT NOT NULL,
    rate        NUMERIC(20, 10) NOT NULL,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX fx_rates_lookup ON fx_rates (base, quote, fetched_at DESC);

CREATE TABLE ingestion_runs (
    id              BIGSERIAL PRIMARY KEY,
    provider_id     BIGINT NOT NULL REFERENCES providers(id),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ,
    status          TEXT NOT NULL DEFAULT 'running',
    services_count  INT NOT NULL DEFAULT 0,
    skus_count      BIGINT NOT NULL DEFAULT 0,
    error           TEXT
);
CREATE INDEX ingestion_runs_provider ON ingestion_runs (provider_id, started_at DESC);
