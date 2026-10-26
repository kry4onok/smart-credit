-- +goose Up
-- Tombstones also serve as a durable queue for document cleanup after commit.
CREATE TABLE deleted_drafts (
    application_id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    deleted_at timestamptz NOT NULL DEFAULT now(),
    documents_cleaned_at timestamptz
);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM deleted_drafts) THEN
        RAISE EXCEPTION 'Cannot discard deletion tombstones and document cleanup jobs';
    END IF;
END $$;
-- +goose StatementEnd
DROP TABLE deleted_drafts;
