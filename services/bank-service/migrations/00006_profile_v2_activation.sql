-- +goose Up
-- The v2 validation and decision paths are now enabled. Catalog publication is
-- still driven by the existing active-product publisher.
ALTER TABLE products DROP CONSTRAINT products_requirements_policy_not_enabled;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM products WHERE requirements_policy_version IS NOT NULL AND is_active) THEN
        RAISE EXCEPTION 'Cannot disable profile v2 while versioned products are active';
    END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE products ADD CONSTRAINT products_requirements_policy_not_enabled
    CHECK (requirements_policy_version IS NULL OR NOT is_active);
