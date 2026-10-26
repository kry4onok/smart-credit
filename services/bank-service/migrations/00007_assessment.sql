-- +goose Up
ALTER TABLE products DROP CONSTRAINT products_requirements_policy_version_check;
ALTER TABLE products ADD CONSTRAINT products_requirements_policy_version_check CHECK (requirements_policy_version IS NULL OR requirements_policy_version IN ('sme-profile-v1','sme-profile-v2'));
ALTER TABLE cases ADD COLUMN assessment_generation bigint NOT NULL DEFAULT 1 CHECK (assessment_generation>0);
ALTER TABLE cases ADD COLUMN assessment_as_of timestamptz NOT NULL DEFAULT now();
ALTER TABLE cases ADD COLUMN assessment_input jsonb NOT NULL DEFAULT '{"schema_version":1,"preferences":{}}';
ALTER TABLE cases ADD COLUMN assessment_sources jsonb;
ALTER TABLE scoring_attempts ADD COLUMN assessment_generation bigint NOT NULL DEFAULT 1;
ALTER TABLE enrichment_results ADD COLUMN assessment_generation bigint NOT NULL DEFAULT 1;
CREATE TABLE assessments (
 id text PRIMARY KEY, case_id uuid NOT NULL REFERENCES cases(id),
 input_revision bigint NOT NULL, assessment_generation bigint NOT NULL,
 input jsonb NOT NULL, result jsonb NOT NULL, public_result jsonb NOT NULL,
 valid_until timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(case_id,assessment_generation)
);
CREATE TABLE assessment_commands (
 case_id uuid NOT NULL REFERENCES cases(id), idempotency_key uuid NOT NULL,
 actor_id uuid NOT NULL, expected_version bigint NOT NULL, generation bigint NOT NULL,
 PRIMARY KEY(case_id,idempotency_key)
);
CREATE INDEX assessments_case_history ON assessments(case_id,assessment_generation DESC);
-- +goose Down
ALTER TABLE products DROP CONSTRAINT products_requirements_policy_version_check;
ALTER TABLE products ADD CONSTRAINT products_requirements_policy_version_check CHECK (requirements_policy_version IS NULL OR requirements_policy_version='sme-profile-v1');
DROP TABLE assessment_commands;
DROP TABLE assessments;
ALTER TABLE enrichment_results DROP COLUMN assessment_generation;
ALTER TABLE scoring_attempts DROP COLUMN assessment_generation;
ALTER TABLE cases DROP COLUMN assessment_sources, DROP COLUMN assessment_input, DROP COLUMN assessment_as_of, DROP COLUMN assessment_generation;
