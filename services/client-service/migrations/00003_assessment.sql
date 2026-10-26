-- +goose Up
ALTER TABLE applications ADD COLUMN assessment_input jsonb NOT NULL DEFAULT '{"schema_version":1,"preferences":{}}';
ALTER TABLE applications ADD COLUMN public_assessment jsonb;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM applications WHERE public_assessment IS NOT NULL OR assessment_input <> '{"schema_version":1,"preferences":{}}'::jsonb)
 THEN RAISE EXCEPTION 'Cannot remove financial inputs or assessment projections'; END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE applications DROP COLUMN public_assessment, DROP COLUMN assessment_input;
