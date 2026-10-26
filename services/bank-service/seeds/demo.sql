-- Demo data for bank_db. Run after all bank migrations, never as a migration.
-- Fixed UUIDs make list/detail examples repeatable. Re-running keeps existing cases;
-- the demo offer expiry is renewed so it remains visible in case detail.
BEGIN;

INSERT INTO products (
    id, version, code, name, description, purpose_tags,
    required_document_types, min_amount_kopecks, max_amount_kopecks,
    min_term_months, max_term_months, indicative_rate_bps, is_active
) VALUES
    ('11111111-1111-4111-8111-111111111111', 1, 'WORKING',
     'Оборотный кредит', 'Пополнение оборотных средств',
     ARRAY['WORKING_CAPITAL'], ARRAY['BANK_STATEMENT'],
     100000000, 5000000000, 6, 36, 1650, true),
    ('22222222-2222-4222-8222-222222222222', 1, 'FLEX',
     'Гибкий кредит', 'Альтернативный продукт для оборотных средств',
     ARRAY['WORKING_CAPITAL', 'EXPANSION'], ARRAY['BANK_STATEMENT'],
     100000000, 3000000000, 6, 24, 1850, true)
ON CONFLICT (id, version) DO NOTHING;


-- Profile-v2 contract fixture. Not published until the requirements engine is enabled.
INSERT INTO products (
 id, version, code, name, description, purpose_tags, required_document_types,
 min_amount_kopecks, max_amount_kopecks, min_term_months, max_term_months,
 indicative_rate_bps, is_active, requirements_policy_version
) VALUES (
 '10000000-0000-4000-8000-000000000021', 1, 'SME_PROFILE_V2',
 'Демо: профиль ИП/ООО', 'Тестовый продукт для v2 матрицы',
 ARRAY['WORKING_CAPITAL'], ARRAY[]::text[], 10000000, 500000000, 6, 36, 1800,
 false, 'sme-profile-v1'
) ON CONFLICT (id, version) DO NOTHING;

INSERT INTO product_rules (
    product_id, product_version, min_company_age_months,
    max_amount_to_revenue_bps, max_overdue_days, rate_grade_a_bps,
    rate_grade_b_bps, offer_ttl_hours
) VALUES
    ('11111111-1111-4111-8111-111111111111', 1, 12, 5000, 30, 1450, 1650, 168),
    ('22222222-2222-4222-8222-222222222222', 1, 12, 4000, 30, 1650, 1850, 168)
ON CONFLICT (product_id, product_version) DO NOTHING;

WITH demo_cases (
    id, org_id, client_user_id, company_inn, company_name, okved,
    registered_at, revenue_kopecks, profit_kopecks, obligations_kopecks,
    amount_kopecks, term_months, status, recommendation, assignee_id,
    public_reason, triggered_rules, created_at
) AS (VALUES
    ('10000000-0000-4000-8000-000000000001'::uuid,
     '20000000-0000-4000-8000-000000000001'::uuid,
     '30000000-0000-4000-8000-000000000001'::uuid,
     '7707083893', 'ООО «Северная пекарня»', '10.71', '2019-04-15'::date,
     840000000, 96000000, 120000000, 300000000, 18,
     'MANUAL_REVIEW', 'MANUAL_REVIEW', NULL::uuid, NULL::text,
     ARRAY['REVIEW_REQUIRED']::text[], now() - interval '4 days'),
    ('10000000-0000-4000-8000-000000000002'::uuid,
     '20000000-0000-4000-8000-000000000002'::uuid,
     '30000000-0000-4000-8000-000000000002'::uuid,
     '7810755500', 'ООО «Балтийский склад»', '52.10', '2016-09-20'::date,
     1500000000, 180000000, 350000000, 500000000, 24,
     'MANUAL_REVIEW', 'MANUAL_REVIEW',
     'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'::uuid, NULL::text,
     ARRAY['LARGE_REQUEST']::text[], now() - interval '3 days'),
    ('10000000-0000-4000-8000-000000000003'::uuid,
     '20000000-0000-4000-8000-000000000003'::uuid,
     '30000000-0000-4000-8000-000000000003'::uuid,
     '5401010012', 'ООО «Сибирский сервис»', '43.21', '2021-02-10'::date,
     520000000, 42000000, 90000000, 220000000, 12,
     'NEEDS_INFO', 'REQUEST_INFO', NULL::uuid,
     'Для продолжения рассмотрения нужна выписка по счёту.',
     ARRAY[]::text[], now() - interval '2 days'),
    ('10000000-0000-4000-8000-000000000004'::uuid,
     '20000000-0000-4000-8000-000000000004'::uuid,
     '30000000-0000-4000-8000-000000000004'::uuid,
     '6671160003', 'ООО «Уральская мастерская»', '25.62', '2018-11-05'::date,
     1100000000, 120000000, 210000000, 600000000, 30,
     'COUNTER_OFFER', 'COUNTER', NULL::uuid,
     'Предлагаем меньшую сумму по другому продукту.',
     ARRAY['LARGE_REQUEST']::text[], now() - interval '1 day'),
    ('10000000-0000-4000-8000-000000000005'::uuid,
     '20000000-0000-4000-8000-000000000005'::uuid,
     '30000000-0000-4000-8000-000000000005'::uuid,
     '7707083893', 'ООО «Северная пекарня» — автооценка', '10.71', '2019-04-15'::date,
     840000000, 96000000, 120000000, 300000000, 18,
     'SCORING', NULL::text, NULL::uuid, NULL::text,
     ARRAY[]::text[], now() - interval '2 hours'),
    ('10000000-0000-4000-8000-000000000006'::uuid,
     '20000000-0000-4000-8000-000000000006'::uuid,
     '30000000-0000-4000-8000-000000000006'::uuid,
     '7707083893', 'ООО «Северная пекарня» — проверка БКИ', '10.71', '2019-04-15'::date,
     840000000, 96000000, 120000000, 300000000, 18,
     'ENRICHMENT', NULL::text, NULL::uuid, NULL::text,
     ARRAY[]::text[], now() - interval '1 hour'),
    ('10000000-0000-4000-8000-000000000007'::uuid,
     '20000000-0000-4000-8000-000000000007'::uuid,
     '30000000-0000-4000-8000-000000000007'::uuid,
     '5401010012', 'ООО «Сибирский сервис» — таймаут БКИ', '43.21', '2021-02-10'::date,
     520000000, 42000000, 90000000, 220000000, 12,
     'ENRICHMENT', NULL::text, NULL::uuid, NULL::text,
     ARRAY[]::text[], now() - interval '30 minutes')
)
INSERT INTO cases (
    id, org_id, client_user_id, product_id, product_version,
    product_name, company_inn, company_name, company_snapshot,
    application_snapshot, documents_snapshot, requested_amount_kopecks,
    requested_term_months, purpose_tag, status, recommendation,
    assignee_id, public_reason, triggered_rules, version, audit_version,
    client_event_version, created_at, updated_at
)
SELECT
    id, org_id, client_user_id, '11111111-1111-4111-8111-111111111111'::uuid,
    1, 'Оборотный кредит', company_inn, company_name,
    jsonb_build_object(
        'inn', company_inn, 'name', company_name, 'okved', okved,
        'registered_at', registered_at,
        'revenue_12m', jsonb_build_object('amount_kopecks', revenue_kopecks, 'currency', 'RUB'),
        'profit_12m', jsonb_build_object('amount_kopecks', profit_kopecks, 'currency', 'RUB'),
        'obligations', jsonb_build_object('amount_kopecks', obligations_kopecks, 'currency', 'RUB')
    ),
    jsonb_build_object(
        'amount', jsonb_build_object('amount_kopecks', amount_kopecks, 'currency', 'RUB'),
        'term_months', term_months, 'purpose', 'WORKING_CAPITAL'
    ),
    jsonb_build_array(jsonb_build_object(
        'id', ('40000000-0000-4000-8000-' || right(id::text, 12))::uuid,
        'document_type', 'BANK_STATEMENT', 'file_name', 'demo-statement.pdf',
        'sha256', repeat('a', 64), 'uploaded_at', created_at,
        'is_mock', true
    )),
    amount_kopecks, term_months, 'WORKING_CAPITAL', status,
    recommendation, assignee_id, public_reason, triggered_rules,
    1, 1, 1, created_at, created_at
FROM demo_cases
ON CONFLICT (id) DO NOTHING;

INSERT INTO enrichment_results (case_id, source, outcome, result, created_at)
SELECT c.id, e.source, 'SUCCESS', e.result, c.created_at + interval '1 hour'
FROM cases AS c
JOIN (VALUES
    ('10000000-0000-4000-8000-000000000001'::uuid, 'COMPANY',
     '{"status":"ACTIVE","registered_at":"2019-04-15","okved":"10.71"}'::jsonb),
    ('10000000-0000-4000-8000-000000000001'::uuid, 'CREDIT_HISTORY',
     '{"total_debt":{"amount_kopecks":120000000,"currency":"RUB"},"overdue_count_12m":0,"max_overdue_days":0}'::jsonb),
    ('10000000-0000-4000-8000-000000000002'::uuid, 'COMPANY',
     '{"status":"ACTIVE","registered_at":"2016-09-20","okved":"52.10"}'::jsonb),
    ('10000000-0000-4000-8000-000000000002'::uuid, 'CREDIT_HISTORY',
     '{"total_debt":{"amount_kopecks":350000000,"currency":"RUB"},"overdue_count_12m":1,"max_overdue_days":12}'::jsonb),
    ('10000000-0000-4000-8000-000000000004'::uuid, 'COMPANY',
     '{"status":"ACTIVE","registered_at":"2018-11-05","okved":"25.62"}'::jsonb),
    ('10000000-0000-4000-8000-000000000004'::uuid, 'CREDIT_HISTORY',
     '{"total_debt":{"amount_kopecks":210000000,"currency":"RUB"},"overdue_count_12m":0,"max_overdue_days":0}'::jsonb),
    ('10000000-0000-4000-8000-000000000005'::uuid, 'COMPANY',
     '{"status":"ACTIVE","registered_at":"2019-04-15","okved":"10.71"}'::jsonb),
    ('10000000-0000-4000-8000-000000000005'::uuid, 'CREDIT_HISTORY',
     '{"total_debt":{"amount_kopecks":120000000,"currency":"RUB"},"overdue_count_12m":0,"max_overdue_days":0}'::jsonb)
) AS e(case_id, source, result) ON e.case_id = c.id
WHERE NOT EXISTS (
    SELECT 1 FROM enrichment_results AS existing
    WHERE existing.case_id = e.case_id AND existing.source = e.source
);

INSERT INTO enrichment_results (case_id, source, outcome, error_code, created_at)
SELECT c.id, 'CREDIT_HISTORY', 'ERROR', 'BUREAU_TIMEOUT',
       c.created_at + interval '1 hour'
FROM cases AS c
WHERE c.id = '10000000-0000-4000-8000-000000000003'
  AND NOT EXISTS (
      SELECT 1 FROM enrichment_results AS existing
      WHERE existing.case_id = c.id AND existing.source = 'CREDIT_HISTORY'
  );

INSERT INTO score_results (
    case_id, score, grade, reasons, features, model_version,
    feature_schema_version, created_at
)
SELECT c.id, s.score, s.grade, s.reasons, '{}'::jsonb,
       'demo-1', 'demo-1', c.created_at + interval '2 hours'
FROM cases AS c
JOIN (VALUES
    ('10000000-0000-4000-8000-000000000001'::uuid, 720, 'B',
     '[{"code":"STABLE_BUSINESS","description":"Компания работает несколько лет","direction":"DECREASES_RISK","contribution":80},{"code":"NO_OVERDUE","description":"Нет просрочек","direction":"DECREASES_RISK","contribution":40}]'::jsonb),
    ('10000000-0000-4000-8000-000000000002'::uuid, 580, 'C',
     '[{"code":"LARGE_REQUEST","description":"Запрошена крупная сумма","direction":"INCREASES_RISK","contribution":90},{"code":"RECENT_OVERDUE","description":"Была недавняя просрочка","direction":"INCREASES_RISK","contribution":35}]'::jsonb),
    ('10000000-0000-4000-8000-000000000004'::uuid, 690, 'B',
     '[{"code":"LARGE_REQUEST","description":"Сумма превышает комфортный диапазон","direction":"INCREASES_RISK","contribution":70},{"code":"STABLE_BUSINESS","description":"Стабильная деятельность","direction":"DECREASES_RISK","contribution":50}]'::jsonb)
) AS s(case_id, score, grade, reasons) ON s.case_id = c.id
WHERE NOT EXISTS (
    SELECT 1 FROM score_results AS existing WHERE existing.case_id = s.case_id
);

INSERT INTO decisions (
    id, case_id, actor_type, outcome, public_reason, product_id,
    product_version, created_at
) VALUES
    ('40000000-0000-4000-8000-000000000003',
     '10000000-0000-4000-8000-000000000003', 'SYSTEM', 'REQUEST_INFO',
     'Для продолжения рассмотрения нужна выписка по счёту.', NULL, NULL,
     now() - interval '1 day'),
    ('40000000-0000-4000-8000-000000000004',
     '10000000-0000-4000-8000-000000000004', 'SYSTEM', 'COUNTER',
     'Предлагаем меньшую сумму по другому продукту.',
     '22222222-2222-4222-8222-222222222222', 1,
     now() - interval '12 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO info_requests (
    id, case_id, decision_id, message, required_fields,
    required_document_types, status, created_at
) VALUES (
    '50000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000003',
    '40000000-0000-4000-8000-000000000003',
    'Приложите актуальную выписку по расчётному счёту.',
    ARRAY[]::text[], ARRAY['BANK_STATEMENT']::text[], 'OPEN',
    now() - interval '1 day'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO offers (
    id, case_id, decision_id, kind, product_id, product_version,
    product_name, amount_kopecks, term_months, rate_bps,
    explanation, status, expires_at, created_at
) VALUES (
    '50000000-0000-4000-8000-000000000004',
    '10000000-0000-4000-8000-000000000004',
    '40000000-0000-4000-8000-000000000004', 'COUNTER_OFFER',
    '22222222-2222-4222-8222-222222222222', 1, 'Гибкий кредит',
    400000000, 24, 1850,
    'Другой продукт с меньшей суммой и сроком.', 'AVAILABLE',
    now() + interval '30 days', now() - interval '12 hours'
)
ON CONFLICT (id) DO UPDATE
SET expires_at = EXCLUDED.expires_at
WHERE offers.status = 'AVAILABLE';

INSERT INTO audit_log (
    id, case_id, case_version, action, actor_type, reason, details, occurred_at
)
SELECT ('60000000-0000-4000-8000-' || right(c.id::text, 12))::uuid,
       c.id, c.version, 'DEMO_SEEDED', 'SYSTEM', 'Демонстрационный сценарий',
       jsonb_build_object('to_status', c.status), c.created_at
FROM cases AS c
WHERE c.id IN (
    '10000000-0000-4000-8000-000000000001',
    '10000000-0000-4000-8000-000000000002',
    '10000000-0000-4000-8000-000000000003',
    '10000000-0000-4000-8000-000000000004',
    '10000000-0000-4000-8000-000000000005',
    '10000000-0000-4000-8000-000000000006',
    '10000000-0000-4000-8000-000000000007'
)
ON CONFLICT (id) DO NOTHING;

COMMIT;
