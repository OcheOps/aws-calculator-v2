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
