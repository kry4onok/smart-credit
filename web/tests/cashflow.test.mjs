import assert from "node:assert/strict";
import test from "node:test";
import { cashFlowPoints, cashFlowScale, cashFlowSegments, firstCashShortfall, historicalCashFlowPoints } from "../src/lib/cashflow.ts";

const row = (period, cfads, existing = 10_000, fresh = 20_000, closing = 50_000) => ({
  period, cfads_kopecks: cfads, existing_debt_payment_kopecks: existing,
  new_debt_payment_kopecks: fresh, closing_cash_kopecks: closing,
  operating_receipts_kopecks: 100_000, evidence_refs: [], quality: "PROJECTED",
});

test("missing months and missing scenario observations are gaps, not zeros or connecting lines", () => {
  const points = cashFlowPoints([row("2026-11", 35_000), row("2027-01", 35_000)], [row("2026-11", 0)]);
  assert.deepEqual(points.map((p) => p.period), ["2026-11", "2026-12", "2027-01"]);
  assert.deepEqual(points[1], { period: "2026-12", base: null, stress: null, payment: null });
  assert.equal(points[2].stress, null);
  assert.deepEqual(cashFlowSegments(points, "base"), [[{ index: 0, value: 35_000 }], [{ index: 2, value: 35_000 }]]);
  assert.deepEqual(cashFlowSegments(points, "stress"), [[{ index: 0, value: 0 }]]);
});

test("payment includes existing and new debt without rounding kopecks", () => {
  const points = cashFlowPoints([row("2026-11", 35_000, 10_123, 20_456)], []);
  assert.equal(points[0].payment, 30_579);
  assert.equal(points[0].stress, null);
});

test("a monthly shortfall is reported even when accumulated cash stays positive", () => {
  const shortfall = firstCashShortfall([row("2026-12", 40_000), row("2026-11", 3_000, 10_000, 20_000, 73_000)]);
  assert.equal(shortfall.period, "2026-11");
  assert.equal(shortfall.shortfall, 27_000);
  assert.equal(shortfall.closing_cash_kopecks, 73_000);
});

test("negative flow increases the reserve needed, while exact coverage has no shortfall", () => {
  assert.equal(firstCashShortfall([row("2026-11", -5_000)]).shortfall, 35_000);
  assert.equal(firstCashShortfall([row("2026-11", 30_000)]), null);
  assert.equal(firstCashShortfall([]), null);
});

test("history does not invent loan payments or connect missing periods", () => {
  const history = [row("2026-11", 0), row("2027-01", -5_000)].map(({ period, cfads_kopecks, operating_receipts_kopecks, evidence_refs }) => ({ period, cfads_kopecks, operating_receipts_kopecks, evidence_refs, quality: "PROVIDED" }));
  const points = historicalCashFlowPoints(history);
  assert.equal(points[0].base, 0);
  assert.equal(points[1].base, null);
  assert.ok(points.every((p) => p.payment === null && p.stress === null));
});

test("axis contains negative flow, payments and zero; an empty or zero plot has a finite range", () => {
  const scale = cashFlowScale(cashFlowPoints([row("2026-11", -5_000), row("2026-12", 350_000)], []));
  assert.ok(scale.min <= -5_000 && scale.max >= 350_000);
  assert.ok(scale.ticks.includes(0));
  for (const points of [[], cashFlowPoints([row("2026-11", 0, 0, 0)], [])]) {
    const s = cashFlowScale(points);
    assert.ok(Number.isFinite(s.max) && s.max > s.min);
    assert.equal(new Set(s.ticks).size, s.ticks.length);
  }
});
