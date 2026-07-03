CREATE TABLE ingestion_price_files (
    id           BIGSERIAL PRIMARY KEY,
    provider_id  BIGINT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    service_code TEXT NOT NULL,
    region_code  TEXT NOT NULL,
    version_id   TEXT NOT NULL,
    etag         TEXT NOT NULL,
    url          TEXT NOT NULL,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider_id, service_code, region_code)
);
CREATE INDEX ingestion_price_files_svc ON ingestion_price_files (service_code);
