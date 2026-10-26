-- +goose Up
-- Product id/version and company/form/document snapshots already live in cases.
-- Keep the draft's policy date separately so later updates cannot move it.
ALTER TABLE cases ADD COLUMN requirements_reference_date date;

-- The selected definitions, including period and accepted types, are frozen with
-- each request. Legacy requests continue to use their existing arrays.
ALTER TABLE info_requests
    ADD COLUMN requirements jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD CONSTRAINT info_requests_requirements_array
        CHECK (jsonb_typeof(requirements) = 'array');

ALTER TABLE info_requests DROP CONSTRAINT info_requests_check;
ALTER TABLE info_requests ADD CONSTRAINT info_requests_nonempty
    CHECK (cardinality(required_fields) + cardinality(required_document_types)
           + jsonb_array_length(requirements) > 0);

-- Accept the new document vocabulary for future product versions. The seeded
-- v2 product keeps this list empty; its requirements come from the policy.
ALTER TABLE products DROP CONSTRAINT products_required_document_types_check;
ALTER TABLE products ADD CONSTRAINT products_required_document_types_check
    CHECK (required_document_types <@ ARRAY[
        'BANK_STATEMENT', 'BUSINESS_PLAN', 'ACCOUNTING_REPORT',
        'MANAGEMENT_REPORT', 'TAX_DECLARATION_USN',
        'TAX_DECLARATION_3NDFL', 'INCOME_LEDGER', 'PATENT',
        'RECEIVABLES_PAYABLES'
    ]::text[]);

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM cases
        WHERE requirements_reference_date IS NOT NULL
           OR company_snapshot ?| ARRAY[
               'profileSchemaVersion', 'profile_schema_version',
               'legalForm', 'legal_form', 'taxPeriods', 'tax_periods',
               'financialPeriod', 'financial_period'
           ]
           OR EXISTS (
               SELECT 1 FROM jsonb_array_elements(documents_snapshot) AS d(doc)
               WHERE doc ? 'requirementId' OR doc ? 'requirement_id'
           )
    ) OR EXISTS (
        SELECT 1 FROM info_requests WHERE requirements <> '[]'::jsonb
    ) OR EXISTS (
        SELECT 1 FROM products
        WHERE NOT (required_document_types <@ ARRAY['BANK_STATEMENT', 'BUSINESS_PLAN']::text[])
    ) THEN
        RAISE EXCEPTION 'Cannot drop profile requirements storage while v2 data exists';
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE products DROP CONSTRAINT products_required_document_types_check;
ALTER TABLE products ADD CONSTRAINT products_required_document_types_check
    CHECK (required_document_types <@ ARRAY['BANK_STATEMENT', 'BUSINESS_PLAN']::text[]);

ALTER TABLE info_requests DROP CONSTRAINT info_requests_nonempty;
ALTER TABLE info_requests DROP CONSTRAINT info_requests_requirements_array;
ALTER TABLE info_requests DROP COLUMN requirements;
ALTER TABLE info_requests ADD CONSTRAINT info_requests_check
    CHECK (cardinality(required_fields) + cardinality(required_document_types) > 0);

ALTER TABLE cases DROP COLUMN requirements_reference_date;
