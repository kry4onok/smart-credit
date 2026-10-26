import type { CashMonth, HistoricalCashMonth } from "../api/types.ts";

export type CashFlowPoint = {
  period: string;
  base: number | null;
  stress: number | null;
  payment: number | null;
};

const monthIndex = (period: string) => Number(period.slice(0, 4)) * 12 + Number(period.slice(5, 7)) - 1;

// Missing observations remain gaps, including months omitted from both scenarios.
export function cashFlowPoints(base: readonly CashMonth[], stress: readonly CashMonth[]): CashFlowPoint[] {
  const periods = [...new Set([...base, ...stress].map((row) => row.period))].sort();
  if (!periods.length) return [];
  const baseByPeriod = new Map(base.map((row) => [row.period, row]));
  const stressByPeriod = new Map(stress.map((row) => [row.period, row]));
  const result: CashFlowPoint[] = [];
  for (let month = monthIndex(periods[0]); month <= monthIndex(periods.at(-1)!); month++) {
    const period = `${Math.floor(month / 12)}-${String(month % 12 + 1).padStart(2, "0")}`;
    const b = baseByPeriod.get(period);
    const s = stressByPeriod.get(period);
    const payments = b ?? s;
    result.push({
      period,
      base: b?.cfads_kopecks ?? null,
      stress: s?.cfads_kopecks ?? null,
      payment: payments ? payments.existing_debt_payment_kopecks + payments.new_debt_payment_kopecks : null,
    });
  }
  return result;
}

export function historicalCashFlowPoints(history: readonly HistoricalCashMonth[]): CashFlowPoint[] {
  // Reuse the calendar alignment, without inventing historical loan payments.
  const points = cashFlowPoints(history.map((row) => ({ ...row, existing_debt_payment_kopecks: 0, new_debt_payment_kopecks: 0, closing_cash_kopecks: 0 })), []);
  return points.map((point) => ({ ...point, payment: null }));
}

export function firstCashShortfall(rows: readonly CashMonth[]) {
  const row = [...rows].sort((a, b) => a.period.localeCompare(b.period)).find((m) => m.cfads_kopecks < m.existing_debt_payment_kopecks + m.new_debt_payment_kopecks);
  return row ? { ...row, shortfall: row.existing_debt_payment_kopecks + row.new_debt_payment_kopecks - row.cfads_kopecks } : null;
}

export function cashFlowSegments(points: readonly CashFlowPoint[], key: "base" | "stress" | "payment") {
  const segments: { index: number; value: number }[][] = [];
  let current: { index: number; value: number }[] = [];
  points.forEach((point, index) => {
    const value = point[key];
    if (value === null) {
      if (current.length) segments.push(current);
      current = [];
    } else current.push({ index, value });
  });
  if (current.length) segments.push(current);
  return segments;
}

export function cashFlowScale(points: readonly CashFlowPoint[]) {
  const values = points.flatMap((point) => [point.base, point.stress, point.payment].filter((value): value is number => value !== null));
  const low = Math.min(0, ...values);
  const high = Math.max(0, ...values);
  const roughStep = Math.max(1, (high - low) / 4);
  const magnitude = 10 ** Math.floor(Math.log10(roughStep));
  const step = ([1, 2, 5, 10].find((factor) => factor * magnitude >= roughStep) ?? 10) * magnitude;
  const min = Math.floor(low / step) * step;
  const max = Math.max(min + step, Math.ceil(high / step) * step);
  const ticks = Array.from({ length: Math.round((max - min) / step) + 1 }, (_, i) => min + i * step);
  return { min, max, ticks };
}
