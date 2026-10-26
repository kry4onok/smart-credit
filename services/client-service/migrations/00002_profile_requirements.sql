-- +goose Up
ALTER TABLE companies
 ALTER COLUMN revenue_12m_kopecks DROP NOT NULL,
 ALTER COLUMN profit_12m_kopecks DROP NOT NULL,
 ALTER COLUMN obligations_kopecks DROP NOT NULL,
 ADD COLUMN profile_schema_version integer CHECK (profile_schema_version = 2),
 ADD COLUMN legal_form text CHECK (legal_form IN ('IP','LLC')),
 ADD COLUMN tax_periods jsonb CHECK (jsonb_typeof(tax_periods) = 'array'),
 ADD COLUMN financial_period jsonb CHECK (jsonb_typeof(financial_period) = 'object'),
 ADD COLUMN financial_basis text CHECK (financial_basis IN ('MANAGEMENT','ACCOUNTING','TAX','UNKNOWN')),
 ADD COLUMN obligations_as_of date;
ALTER TABLE products ADD COLUMN requirements_policy_version text;
ALTER TABLE applications ADD COLUMN requirements_reference_date date,
 ADD COLUMN bank_requirements_plan jsonb CHECK (jsonb_typeof(bank_requirements_plan) = 'object');
ALTER TABLE application_documents
 ADD COLUMN requirement_id text,
 ADD COLUMN coverage_from date,
 ADD COLUMN coverage_to date,
 ADD CONSTRAINT document_coverage CHECK (
   (requirement_id IS NULL AND coverage_from IS NULL AND coverage_to IS NULL) OR
   (requirement_id IS NOT NULL AND btrim(requirement_id) <> '' AND coverage_from IS NOT NULL AND coverage_to IS NOT NULL AND coverage_from <= coverage_to)
 );
ALTER TABLE application_documents DROP CONSTRAINT application_documents_document_type_check;
ALTER TABLE application_documents ADD CONSTRAINT application_documents_document_type_check CHECK (document_type IN (
 'BANK_STATEMENT','BUSINESS_PLAN','ACCOUNTING_REPORT','MANAGEMENT_REPORT','TAX_DECLARATION_USN','TAX_DECLARATION_3NDFL','INCOME_LEDGER','PATENT','RECEIVABLES_PAYABLES'));
ALTER TABLE info_requests ADD COLUMN requirements jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(requirements) = 'array');
ALTER TABLE info_requests DROP CONSTRAINT info_requests_check;
ALTER TABLE info_requests ADD CONSTRAINT info_requests_check CHECK (
 (jsonb_array_length(requirements) > 0 AND cardinality(required_fields) + cardinality(required_document_types) = 0) OR
 (jsonb_array_length(requirements) = 0 AND cardinality(required_fields) + cardinality(required_document_types) > 0)
);

ALTER TABLE products DROP CONSTRAINT products_required_document_types_check;
ALTER TABLE products ADD CONSTRAINT products_required_document_types_check CHECK (required_document_types <@ ARRAY['BANK_STATEMENT','BUSINESS_PLAN','ACCOUNTING_REPORT','MANAGEMENT_REPORT','TAX_DECLARATION_USN','TAX_DECLARATION_3NDFL','INCOME_LEDGER','PATENT','RECEIVABLES_PAYABLES']::text[] AND array_position(required_document_types,NULL) IS NULL);
ALTER TABLE info_requests DROP CONSTRAINT info_requests_required_document_types_check;
ALTER TABLE info_requests ADD CONSTRAINT info_requests_required_document_types_check CHECK (required_document_types <@ ARRAY['BANK_STATEMENT','BUSINESS_PLAN','ACCOUNTING_REPORT','MANAGEMENT_REPORT','TAX_DECLARATION_USN','TAX_DECLARATION_3NDFL','INCOME_LEDGER','PATENT','RECEIVABLES_PAYABLES']::text[] AND array_position(required_document_types,NULL) IS NULL);

-- +goose Down
-- A rollback must not silently discard v2 data or restore NOT NULL over missing values.
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM companies WHERE profile_schema_version IS NOT NULL OR legal_form IS NOT NULL OR tax_periods IS NOT NULL OR financial_period IS NOT NULL OR financial_basis IS NOT NULL OR obligations_as_of IS NOT NULL OR revenue_12m_kopecks IS NULL OR profit_12m_kopecks IS NULL OR obligations_kopecks IS NULL)
 OR EXISTS (SELECT 1 FROM products WHERE requirements_policy_version IS NOT NULL OR NOT (required_document_types <@ ARRAY['BANK_STATEMENT','BUSINESS_PLAN']::text[]))
 OR EXISTS (SELECT 1 FROM applications WHERE requirements_reference_date IS NOT NULL OR bank_requirements_plan IS NOT NULL OR company_snapshot ? 'profile_schema_version')
 OR EXISTS (SELECT 1 FROM application_documents WHERE requirement_id IS NOT NULL OR document_type NOT IN ('BANK_STATEMENT','BUSINESS_PLAN'))
 OR EXISTS (SELECT 1 FROM info_requests WHERE requirements <> '[]'::jsonb OR NOT (required_document_types <@ ARRAY['BANK_STATEMENT','BUSINESS_PLAN']::text[]))
 THEN RAISE EXCEPTION 'Cannot roll back profile migration while v2 data exists'; END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE info_requests DROP CONSTRAINT info_requests_check;
ALTER TABLE info_requests ADD CONSTRAINT info_requests_check CHECK (cardinality(required_fields) + cardinality(required_document_types) > 0);
ALTER TABLE info_requests DROP COLUMN requirements;
ALTER TABLE application_documents DROP CONSTRAINT document_coverage,
 DROP COLUMN requirement_id, DROP COLUMN coverage_from, DROP COLUMN coverage_to;
ALTER TABLE application_documents DROP CONSTRAINT application_documents_document_type_check;
ALTER TABLE application_documents ADD CONSTRAINT application_documents_document_type_check CHECK (document_type IN ('BANK_STATEMENT','BUSINESS_PLAN'));
ALTER TABLE applications DROP COLUMN requirements_reference_date, DROP COLUMN bank_requirements_plan;
ALTER TABLE products DROP COLUMN requirements_policy_version;
ALTER TABLE companies DROP COLUMN profile_schema_version, DROP COLUMN legal_form, DROP COLUMN tax_periods, DROP COLUMN financial_period, DROP COLUMN financial_basis, DROP COLUMN obligations_as_of,
 ALTER COLUMN revenue_12m_kopecks SET NOT NULL, ALTER COLUMN profit_12m_kopecks SET NOT NULL, ALTER COLUMN obligations_kopecks SET NOT NULL;

ALTER TABLE products DROP CONSTRAINT products_required_document_types_check;
ALTER TABLE products ADD CONSTRAINT products_required_document_types_check CHECK (required_document_types <@ ARRAY['BANK_STATEMENT','BUSINESS_PLAN']::text[] AND array_position(required_document_types,NULL) IS NULL);

ALTER TABLE info_requests DROP CONSTRAINT info_requests_required_document_types_check;
ALTER TABLE info_requests ADD CONSTRAINT info_requests_required_document_types_check CHECK (required_document_types <@ ARRAY['BANK_STATEMENT','BUSINESS_PLAN']::text[] AND array_position(required_document_types,NULL) IS NULL);
