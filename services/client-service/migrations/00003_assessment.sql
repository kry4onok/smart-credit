-- +goose Up
ALTER TABLE applications ADD COLUMN assessment_input jsonb NOT NULL DEFAULT '{"schema_version":1,"preferences":{}}';
ALTER TABLE applications ADD COLUMN public_assessment jsonb;
-- +goose Down
ALTER TABLE applications DROP COLUMN public_assessment, DROP COLUMN assessment_input;
