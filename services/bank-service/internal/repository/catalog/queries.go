package catalogrepository

const ensureStateQuery = `
INSERT INTO catalog_publication (singleton, version)
VALUES (true, 0)
ON CONFLICT (singleton) DO NOTHING`

const lockStateQuery = `
SELECT version, content_hash
FROM catalog_publication
WHERE singleton = true
FOR UPDATE`

const listProductsQuery = `
SELECT id, version, code, name, description, purpose_tags,
       required_document_types, min_amount_kopecks, max_amount_kopecks,
       min_term_months, max_term_months, indicative_rate_bps
FROM products
WHERE is_active
ORDER BY code, id`

const insertOutboxQuery = `
INSERT INTO outbox (event_id, topic, aggregate_id, aggregate_version, event_type, payload)
VALUES ($1, 'bank.events', 'catalog', $2, 'ProductCatalogPublished', $3)`

const updateStateQuery = `
UPDATE catalog_publication
SET version = $1, content_hash = $2
WHERE singleton = true`
