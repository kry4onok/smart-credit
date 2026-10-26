-- +goose Up
-- Contract foundation only. Activation requires the separate requirements engine.
ALTER TABLE products ADD COLUMN requirements_policy_version text;
ALTER TABLE products ADD CONSTRAINT products_requirements_policy_known
    CHECK (requirements_policy_version IS NULL OR requirements_policy_version = 'sme-profile-v1');
ALTER TABLE products ADD CONSTRAINT products_requirements_policy_not_enabled
    CHECK (requirements_policy_version IS NULL OR NOT is_active);

-- +goose Down
-- Preserve any versioned product rather than silently reverting its policy.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM products WHERE requirements_policy_version IS NOT NULL) THEN
        RAISE EXCEPTION 'Cannot drop policy metadata while versioned products exist';
    END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE products DROP CONSTRAINT products_requirements_policy_not_enabled;
ALTER TABLE products DROP CONSTRAINT products_requirements_policy_known;
ALTER TABLE products DROP COLUMN requirements_policy_version;
