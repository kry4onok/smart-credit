import type { CashMonth, HistoricalCashMonth } from "../api/types";

const W = 760;
const H = 260;
const PAD = { top: 16, right: 12, bottom: 34, left: 64 };

const monthFmt = new Intl.DateTimeFormat("ru-RU", { month: "short" });
const yearFmt = new Intl.DateTimeFormat("ru-RU", { year: "2-digit" });

function periodLabel(period: string): string {
  const d = new Date(`${period.slice(0, 7)}-01T00:00:00Z`);
  return Number.isNaN(d.getTime()) ? period : `${monthFmt.format(d).replace(".", "")} ${yearFmt.format(d)}`;
}

function axisMoney(kopecks: number): string {
  const rub = kopecks / 100;
  const abs = Math.abs(rub);
  if (abs >= 1_000_000) return `${(rub / 1_000_000).toLocaleString("ru-RU", { maximumFractionDigits: 1 })} млн`;
  if (abs >= 1_000) return `${Math.round(rub / 1_000).toLocaleString("ru-RU")} тыс`;
  return Math.round(rub).toLocaleString("ru-RU");
}

function tooltipMoney(kopecks: number): string {
  return `${Math.round(kopecks / 100).toLocaleString("ru-RU")} ₽`;
}

type Props = {
  history: HistoricalCashMonth[] | null | undefined;
  base: CashMonth[] | null | undefined;
  stress?: CashMonth[] | null | undefined;
  // Show loan payments (existing + new) as a line over the forecast.
  showPayments?: boolean;
  caption: string;
};

// CashFlowChart shows the monthly operating cash flow the bank relies on: facts, the base forecast,
// the stress scenario and, for a loan option, the payments it must cover.
export function CashFlowChart({ history, base, stress, showPayments = true, caption }: Props) {
  const past = history ?? [];
  const future = base ?? [];
  const stressByPeriod = new Map((stress ?? []).map((m) => [m.period, m]));
  const months = [
    ...past.map((m) => ({ period: m.period, cfads: m.cfads_kopecks, kind: "fact" as const, payment: null as number | null, stress: null as number | null })),
    ...future.map((m) => ({
      period: m.period,
      cfads: m.cfads_kopecks,
      kind: "forecast" as const,
      payment: m.existing_debt_payment_kopecks + m.new_debt_payment_kopecks,
      stress: stressByPeriod.get(m.period)?.cfads_kopecks ?? null,
    })),
  ];
  if (months.length === 0) return <p className="muted">Помесячных данных о денежном потоке пока нет.</p>;

  const values = months.flatMap((m) => [m.cfads, m.payment ?? 0, m.stress ?? 0]);
  const max = Math.max(1, ...values);
  const min = Math.min(0, ...values);
  const plotW = W - PAD.left - PAD.right;
  const plotH = H - PAD.top - PAD.bottom;
  const y = (v: number) => PAD.top + ((max - v) / (max - min)) * plotH;
  const step = plotW / months.length;
  const barW = Math.max(3, Math.min(22, step * 0.62));
  const x = (i: number) => PAD.left + step * i + step / 2;
  const ticks = [max, max / 2, 0, ...(min < 0 ? [min] : [])];
  const labelEvery = Math.ceil(months.length / 12);
  const hasPayments = showPayments && months.some((m) => (m.payment ?? 0) > 0);
  const hasStress = months.some((m) => m.stress !== null);
  const line = (key: "payment" | "stress") =>
    months
      .map((m, i) => (m[key] === null ? null : `${x(i)},${y(m[key]!)}`))
      .filter(Boolean)
      .join(" ");
  const firstForecast = months.findIndex((m) => m.kind === "forecast");

  return (
    <figure className="cashflow">
      <figcaption>{caption}</figcaption>
      <div className="chart-legend">
        {past.length > 0 && (
          <span>
            <i className="key key-fact" /> Денежный поток, факт
          </span>
        )}
        {future.length > 0 && (
          <span>
            <i className="key key-forecast" /> Прогноз банка
          </span>
        )}
        {hasStress && (
          <span>
            <i className="key key-stress" /> Если выручка упадёт на 20%
          </span>
        )}
        {hasPayments && (
          <span>
            <i className="key key-payment" /> Платежи по кредитам
          </span>
        )}
      </div>
      {/* On phones the chart keeps its scale and scrolls sideways instead of shrinking its labels. */}
      <div className="chart-scroll">
        <svg viewBox={`0 0 ${W} ${H}`} className="chart" role="img" aria-label={`${caption}: ${months.length} месяцев`}>
          {ticks.map((t) => (
            <g key={t}>
              <line x1={PAD.left} x2={W - PAD.right} y1={y(t)} y2={y(t)} className={t === 0 ? "grid zero" : "grid"} />
              <text x={PAD.left - 8} y={y(t) + 4} className="tick" textAnchor="end">
                {axisMoney(t)}
              </text>
            </g>
          ))}
          {firstForecast > 0 && (
            <g>
              <line x1={x(firstForecast) - step / 2} x2={x(firstForecast) - step / 2} y1={PAD.top} y2={H - PAD.bottom} className="divider" />
              <text x={x(firstForecast) - step / 2 + 6} y={PAD.top + 10} className="tick">
                прогноз
              </text>
            </g>
          )}
          {months.map((m, i) => {
            const top = y(Math.max(m.cfads, 0));
            const h = Math.max(1, Math.abs(y(m.cfads) - y(0)));
            return (
              <rect key={m.period + m.kind} x={x(i) - barW / 2} y={m.cfads >= 0 ? top : y(0)} width={barW} height={h} rx={2} className={`bar bar-${m.kind}${m.cfads < 0 ? " bar-neg" : ""}`}>
                <title>
                  {periodLabel(m.period)}: поток {tooltipMoney(m.cfads)}
                  {m.payment ? `, платежи ${tooltipMoney(m.payment)}` : ""}
                  {m.stress !== null ? `, в стрессе ${tooltipMoney(m.stress)}` : ""}
                </title>
              </rect>
            );
          })}
          {hasStress && <polyline points={line("stress")} className="line-stress" />}
          {hasPayments && <polyline points={line("payment")} className="line-payment" />}
          {months.map((m, i) =>
            i % labelEvery === 0 ? (
              <text key={`l${m.period}${m.kind}`} x={x(i)} y={H - PAD.bottom + 16} className="tick" textAnchor="middle">
                {periodLabel(m.period)}
              </text>
            ) : null,
          )}
        </svg>
      </div>
    </figure>
  );
}

// CoverageBar compares a payment coverage ratio with the bank's thresholds (×1.3 base, ×1.1 stress).
export function CoverageBar({ label, bps, threshold }: { label: string; bps: number; threshold: number }) {
  const ratio = bps / 10000;
  const cap = 3;
  const pos = Math.min(ratio, cap) / cap;
  const ok = ratio >= threshold;
  return (
    <div className="coverage">
      <span className="coverage-label">{label}</span>
      <span className="coverage-track" aria-hidden>
        <span className="coverage-threshold" style={{ left: `${(threshold / cap) * 100}%` }} />
        <span className={`coverage-fill ${ok ? "ok" : "low"}`} style={{ width: `${pos * 100}%` }} />
      </span>
      <span className={`coverage-value ${ok ? "ok" : "low"}`}>
        ×{ratio.toLocaleString("ru-RU", { maximumFractionDigits: 2 })}
        <small>{ok ? `норма от ×${threshold.toLocaleString("ru-RU")}` : `ниже ×${threshold.toLocaleString("ru-RU")}`}</small>
      </span>
    </div>
  );
}
