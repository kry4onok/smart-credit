-- +goose Up
ALTER TABLE products DROP CONSTRAINT products_purpose_tags_check;
ALTER TABLE products ADD CONSTRAINT products_purpose_tags_check CHECK (
 cardinality(purpose_tags)>0 AND purpose_tags <@ ARRAY['WORKING_CAPITAL','EQUIPMENT','EXPANSION','OTHER']::text[]
 AND array_position(purpose_tags,NULL) IS NULL);
ALTER TABLE cases DROP CONSTRAINT cases_purpose_tag_check;
ALTER TABLE cases ADD CONSTRAINT cases_purpose_tag_check CHECK (purpose_tag IN ('WORKING_CAPITAL','EQUIPMENT','EXPANSION','OTHER'));

-- +goose Down
-- These checks intentionally refuse rollback while OTHER data exists.
ALTER TABLE products DROP CONSTRAINT products_purpose_tags_check;
ALTER TABLE products ADD CONSTRAINT products_purpose_tags_check CHECK (
 cardinality(purpose_tags)>0 AND purpose_tags <@ ARRAY['WORKING_CAPITAL','EQUIPMENT','EXPANSION']::text[]
 AND array_position(purpose_tags,NULL) IS NULL);
ALTER TABLE cases DROP CONSTRAINT cases_purpose_tag_check;
ALTER TABLE cases ADD CONSTRAINT cases_purpose_tag_check CHECK (purpose_tag IN ('WORKING_CAPITAL','EQUIPMENT','EXPANSION'));
