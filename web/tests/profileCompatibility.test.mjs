import test from "node:test";
import assert from "node:assert/strict";
import { hasLegacyFinancials, hasProfileV2Fields } from "../src/lib/profileCompatibility.ts";

test("missing financials are not a known zero", () => {
  assert.equal(hasLegacyFinancials({}), false);
  const zero = { amount_kopecks: 0, currency: "RUB" };
  assert.equal(hasLegacyFinancials({ revenue_12m: zero, profit_12m: zero, obligations: zero }), true);
});

test("v2 metadata cannot silently go through legacy mock processing", () => {
  assert.equal(hasProfileV2Fields({}), false);
  assert.equal(hasProfileV2Fields({ profile_schema_version: 2 }), true);
  assert.equal(hasProfileV2Fields({ tax_periods: [] }), true);
});
