-- +goose Up
ALTER TABLE cases
    ADD COLUMN scoring_input_revision bigint NOT NULL DEFAULT 1
        CHECK (scoring_input_revision > 0),
    ADD COLUMN current_scoring_attempt_id uuid;

CREATE TABLE scoring_attempts (
    id uuid PRIMARY KEY,
    case_id uuid NOT NULL REFERENCES cases (id),
    input_revision bigint NOT NULL CHECK (input_revision > 0),
    as_of_date date NOT NULL,
    request jsonb NOT NULL CHECK (jsonb_typeof(request) = 'object'),
    status text NOT NULL DEFAULT 'PENDING' CHECK (status IN (
        'PENDING', 'ASSESSED', 'INSUFFICIENT_DATA', 'UNSUPPORTED_INPUT', 'TECHNICAL_ERROR'
    )),
    response jsonb,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    UNIQUE (case_id, id),
    CHECK (
        (status = 'PENDING' AND response IS NULL AND error_code IS NULL AND completed_at IS NULL)
        OR (status IN ('ASSESSED', 'INSUFFICIENT_DATA', 'UNSUPPORTED_INPUT')
            AND response IS NOT NULL AND jsonb_typeof(response) = 'object'
            AND error_code IS NULL AND completed_at IS NOT NULL)
        OR (status = 'TECHNICAL_ERROR' AND response IS NULL
            AND error_code IS NOT NULL AND error_code <> '' AND completed_at IS NOT NULL)
    )
);

CREATE INDEX scoring_attempts_case_idx
    ON scoring_attempts (case_id, created_at DESC, id DESC);

ALTER TABLE cases
    ADD CONSTRAINT cases_current_scoring_attempt_fk
    FOREIGN KEY (id, current_scoring_attempt_id)
    REFERENCES scoring_attempts (case_id, id);

ALTER TABLE score_results
    ADD COLUMN attempt_id uuid UNIQUE REFERENCES scoring_attempts (id);

-- +goose Down
ALTER TABLE score_results DROP COLUMN attempt_id;
ALTER TABLE cases DROP CONSTRAINT cases_current_scoring_attempt_fk;
DROP TABLE scoring_attempts;
ALTER TABLE cases
    DROP COLUMN current_scoring_attempt_id,
    DROP COLUMN scoring_input_revision;
