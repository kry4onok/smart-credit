-- +goose Up
-- Serialize catalog snapshots and keep their version even after outbox cleanup.
CREATE TABLE catalog_publication (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    version bigint NOT NULL DEFAULT 0 CHECK (version >= 0),
    content_hash bytea CHECK (content_hash IS NULL OR octet_length(content_hash) = 32)
);

-- At most one active version of a product can appear in a full catalog.
CREATE UNIQUE INDEX products_one_active_id_idx ON products (id) WHERE is_active;

-- +goose Down
DROP INDEX products_one_active_id_idx;
DROP TABLE catalog_publication;
