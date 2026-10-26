import assert from "node:assert/strict";
import test from "node:test";
import { latestDocuments, validateInfoValue } from "../src/lib/applicationValidation.ts";

test("missing and invalid bank-request values are blocked; valid zeros are preserved", () => {
  for (const value of [undefined, null, ""]) assert.ok(validateInfoValue("COMPANY_REVENUE_12M", value));
  for (const value of [NaN, Infinity, 1.5, Number.MAX_SAFE_INTEGER + 1, "100"]) assert.ok(validateInfoValue("APPLICATION_AMOUNT", value));
  assert.ok(validateInfoValue("APPLICATION_AMOUNT", 0));
  assert.ok(validateInfoValue("COMPANY_OBLIGATIONS", -1));
  for (const field of ["COMPANY_REVENUE_12M", "COMPANY_PROFIT_12M", "COMPANY_OBLIGATIONS"]) assert.equal(validateInfoValue(field, 0), null);
  assert.equal(validateInfoValue("COMPANY_PROFIT_12M", -100), null);
  assert.equal(validateInfoValue("APPLICATION_AMOUNT", 100), null);
});

test("term is a positive integer; purpose must be selected explicitly", () => {
  for (const value of [0, -1, 1.5, 2147483648]) assert.ok(validateInfoValue("APPLICATION_TERM_MONTHS", value));
  assert.equal(validateInfoValue("APPLICATION_TERM_MONTHS", 12), null);
  assert.ok(validateInfoValue("APPLICATION_PURPOSE", ""));
  assert.ok(validateInfoValue("APPLICATION_PURPOSE", "UNKNOWN"));
  for (const value of ["WORKING_CAPITAL", "EQUIPMENT", "EXPANSION", "OTHER"]) assert.equal(validateInfoValue("APPLICATION_PURPOSE", value), null);
});

test("attachments survive reload and retain every required type", () => {
  const document = (id, type, at) => ({ id, document_type: type, uploaded_at: at });
  const old = document("old", "BANK_STATEMENT", "2026-10-25T12:00:00Z");
  const recent = document("recent", "BANK_STATEMENT", "2026-10-25T12:00:00.123Z");
  const plan = document("plan", "BUSINESS_PLAN", "2026-10-25T11:00:00Z");
  const serverDocuments = [recent, plan, old];
  assert.deepEqual(latestDocuments(serverDocuments), [recent, plan]);
  assert.deepEqual(latestDocuments(JSON.parse(JSON.stringify(serverDocuments))), [recent, plan]);
  assert.deepEqual(latestDocuments([]), []);
  assert.equal(serverDocuments.length, 3);
});
