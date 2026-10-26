import assert from "node:assert/strict";
import test from "node:test";
import { validateInfoValue, validatePurposeDescription } from "../src/lib/applicationValidation.ts";

test("OTHER is a valid purpose and requires a nonblank explanation", () => {
  assert.equal(validateInfoValue("APPLICATION_PURPOSE", "OTHER"), null);
  assert.match(validatePurposeDescription("OTHER", "   "), /опишите/);
  assert.equal(validatePurposeDescription("OTHER", "Ремонт помещения и закупка сырья"), null);
});

test("explanation is optional for listed purposes but capped at 1000 characters", () => {
  assert.equal(validatePurposeDescription("EQUIPMENT", undefined), null);
  assert.equal(validatePurposeDescription("EQUIPMENT", "а".repeat(1000)), null);
  assert.match(validatePurposeDescription("EQUIPMENT", "а".repeat(1001)), /1000/);
});
