ALTER TABLE estimate_items DROP CONSTRAINT IF EXISTS estimate_items_line_type_check;
DROP INDEX IF EXISTS estimate_items_service_key;
ALTER TABLE estimate_items
    DROP COLUMN IF EXISTS components_snapshot,
    DROP COLUMN IF EXISTS user_input_snapshot,
    DROP COLUMN IF EXISTS service_key,
    DROP COLUMN IF EXISTS line_type;
