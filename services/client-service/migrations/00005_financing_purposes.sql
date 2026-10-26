-- +goose Up
ALTER TABLE products DROP CONSTRAINT products_purpose_tags_check;
ALTER TABLE products ADD CONSTRAINT products_purpose_tags_check CHECK (
 cardinality(purpose_tags)>0 AND purpose_tags <@ ARRAY['WORKING_CAPITAL','EQUIPMENT','EXPANSION','OTHER']::text[]
 AND array_position(purpose_tags,NULL) IS NULL);
ALTER TABLE applications DROP CONSTRAINT applications_purpose_tag_check;
ALTER TABLE applications ADD CONSTRAINT applications_purpose_tag_check CHECK (purpose_tag IN ('WORKING_CAPITAL','EQUIPMENT','EXPANSION','OTHER'));
ALTER TABLE applications ADD COLUMN purpose_description text NOT NULL DEFAULT '' CHECK (char_length(purpose_description) <= 1000);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM applications WHERE purpose_description <> '') THEN
  RAISE EXCEPTION 'Cannot discard purpose descriptions';
 END IF;
END $$;
-- +goose StatementEnd
-- These checks intentionally refuse rollback while OTHER data exists.
ALTER TABLE products DROP CONSTRAINT products_purpose_tags_check;
ALTER TABLE products ADD CONSTRAINT products_purpose_tags_check CHECK (
 cardinality(purpose_tags)>0 AND purpose_tags <@ ARRAY['WORKING_CAPITAL','EQUIPMENT','EXPANSION']::text[]
 AND array_position(purpose_tags,NULL) IS NULL);
ALTER TABLE applications DROP CONSTRAINT applications_purpose_tag_check;
ALTER TABLE applications ADD CONSTRAINT applications_purpose_tag_check CHECK (purpose_tag IN ('WORKING_CAPITAL','EQUIPMENT','EXPANSION'));
ALTER TABLE applications DROP COLUMN purpose_description;
