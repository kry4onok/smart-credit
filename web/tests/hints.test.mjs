import assert from "node:assert/strict";
import test from "node:test";
import { annuity, loanHints, principalFor } from "../src/lib/hints.ts";

const base = {
  rateBps: 1600,
  revenueKopecks: 24_000_000_00,
  obligationsKopecks: 1_000_000_00,
  periodMonths: 12,
  registeredAt: "2020-01-01",
  today: new Date("2026-10-26T00:00:00Z"),
};

test("principalFor inverts the annuity", () => {
  const payment = annuity(1_000_000_00, 24, 1600);
  assert.ok(Math.abs(principalFor(payment, 24, 1600) - 1_000_000_00) < 1);
});

test("comfortable request has a good coverage hint", () => {
  const hints = loanHints({ ...base, amountKopecks: 1_000_000_00, termMonths: 24, profitKopecks: 2_400_000_00 });
  assert.equal(hints.find((h) => h.id === "coverage").level, "good");
});

test("payment above profit suggests a smaller amount the bank can cover ×1.3", () => {
  const hints = loanHints({ ...base, amountKopecks: 5_000_000_00, termMonths: 12, profitKopecks: 2_400_000_00 });
  const coverage = hints.find((h) => h.id === "coverage");
  assert.equal(coverage.level, "bad");
  const suggested = coverage.fix.amountKopecks;
  assert.ok(suggested > 0 && suggested % 100_000_00 === 0);
  assert.ok(2_400_000_00 / 12 / annuity(suggested, 12, 1600) >= 1.3);
});

test("young business and missing financials are flagged", () => {
  const hints = loanHints({ ...base, registeredAt: "2026-03-01", amountKopecks: 1_000_000_00, termMonths: 12, profitKopecks: undefined, revenueKopecks: undefined });
  assert.equal(hints[0].id, "age");
  assert.equal(hints[0].level, "bad");
  assert.ok(hints.some((h) => h.id === "financials"));
});
