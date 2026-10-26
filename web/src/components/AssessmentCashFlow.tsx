import { useEffect, useId, useRef, useState } from "react";
import type { Candidate, CashMonth, FinancialSnapshot } from "../api/types";
import { cashFlowPoints, cashFlowScale, cashFlowSegments, firstCashShortfall, historicalCashFlowPoints, type CashFlowPoint } from "../lib/cashflow";
import { money, rub } from "../lib/format";
import { CoverageBar } from "./CashFlowChart";

const periodFormat = new Intl.DateTimeFormat("ru-RU", { month: "short", year: "2-digit", timeZone: "UTC" });
const fullPeriodFormat = new Intl.DateTimeFormat("ru-RU", { month: "long", year: "numeric", timeZone: "UTC" });
const periodDate = (period: string) => new Date(`${period}-01T00:00:00Z`);
const periodLabel = (period: string) => periodFormat.format(periodDate(period)).replace(/\./g, "");
const fullPeriod = (period: string) => fullPeriodFormat.format(periodDate(period));
const amount = (value: number | null) => value === null ? "Нет данных" : money(rub(value), true);

function CashFlowTimeline({ points, historical = false }: { points: CashFlowPoint[]; historical?: boolean }) {
  const container = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(640);
  const [selectedPeriod, setSelectedPeriod] = useState<string | null>(null);
  const descriptionId = useId();
  const hasPoints = points.length > 0;
  useEffect(() => {
    const element = container.current;
    if (!element) return;
    const observer = new ResizeObserver(([entry]) => {
      if (entry.contentRect.width > 0) setWidth(entry.contentRect.width);
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [hasPoints]);

  if (!points.length) return <p className="muted">Помесячных данных пока нет.</p>;
  const height = 260;
  const pad = { top: 24, right: 16, bottom: 32, left: 56 };
  const scale = cashFlowScale(points);
  const x = (index: number) => pad.left + (points.length === 1 ? 0.5 : index / (points.length - 1)) * Math.max(1, width - pad.left - pad.right);
  const y = (value: number) => pad.top + (scale.max - value) / (scale.max - scale.min) * (height - pad.top - pad.bottom);
  const maxLabels = Math.max(2, Math.floor((width - pad.left - pad.right) / 90));
  const labelIndexes = new Set(Array.from({ length: Math.min(maxLabels, points.length) }, (_, i) => Math.round(i * (points.length - 1) / Math.max(1, Math.min(maxLabels, points.length) - 1))));
  const series = [
    { key: "base" as const, label: historical ? "Исторический поток" : "Базовый поток" },
    { key: "stress" as const, label: "Поток в стрессе" },
    { key: "payment" as const, label: "Все кредитные платежи" },
  ].filter((series) => points.some((point) => point[series.key] !== null));
  const selected = points.find((point) => point.period === selectedPeriod) ?? points[0];
  const selectedIndex = points.indexOf(selected);
  const unit = Math.max(Math.abs(scale.min), Math.abs(scale.max)) >= 100_000_000 ? 100_000_000 : 100_000;
  const tickLabel = (value: number) => (value / unit).toLocaleString("ru-RU", { maximumFractionDigits: 3 });

  return (
    <div className="monthly-cashflow" ref={container}>
      <div className="monthly-legend">
        {series.map((s) => <span key={s.key}><i className={`monthly-key monthly-${s.key}`} />{s.label}</span>)}
      </div>
      <svg className="monthly-plot" viewBox={`0 0 ${width} ${height}`} role="img" aria-label={historical ? "Исторический денежный поток по месяцам" : "Поток и все кредитные платежи по месяцам"} aria-describedby={descriptionId}
        onPointerMove={(event) => {
          const bounds = event.currentTarget.getBoundingClientRect();
          const index = Math.round((event.clientX - bounds.left - pad.left) / Math.max(1, width - pad.left - pad.right) * (points.length - 1));
          setSelectedPeriod(points[Math.max(0, Math.min(points.length - 1, index))].period);
        }}>
        <desc id={descriptionId}>Значения в рублях за месяц. Пропуски не соединяются линией. Точные суммы доступны в выборе месяца и таблице под графиком.</desc>
        <text x={pad.left} y={12} className="monthly-tick">{unit === 100_000_000 ? "млн ₽ / месяц" : "тыс. ₽ / месяц"}</text>
        {scale.ticks.map((tick) => <g key={tick}>
          <line x1={pad.left} x2={width - pad.right} y1={y(tick)} y2={y(tick)} className={tick === 0 ? "monthly-grid monthly-zero" : "monthly-grid"} />
          <text x={pad.left - 8} y={y(tick) + 4} textAnchor="end" className="monthly-tick">{tickLabel(tick)}</text>
        </g>)}
        <line x1={x(selectedIndex)} x2={x(selectedIndex)} y1={pad.top} y2={height - pad.bottom} className="monthly-guide" />
        {series.map((s) => <g key={s.key} className={`monthly-series monthly-${s.key}`}>
          {cashFlowSegments(points, s.key).map((segment, index) => <g key={index}>
            <polyline points={segment.map((point) => `${x(point.index)},${y(point.value)}`).join(" ")} />
            {segment.filter((point) => points.length <= 18 || point.index === selectedIndex || segment.length === 1).map((point) => <circle key={point.index} cx={x(point.index)} cy={y(point.value)} r={point.index === selectedIndex ? 4 : 2.5} />)}
          </g>)}
        </g>)}
        {points.map((point, index) => labelIndexes.has(index) ? <text key={point.period} x={x(index)} y={height - 9} textAnchor={points.length === 1 ? "middle" : index === 0 ? "start" : index === points.length - 1 ? "end" : "middle"} className="monthly-tick">{periodLabel(point.period)}</text> : null)}
      </svg>
      <div className="monthly-inspect">
        <label>Месяц<select value={selected.period} onChange={(event) => setSelectedPeriod(event.target.value)}>{points.map((point) => <option key={point.period} value={point.period}>{fullPeriod(point.period)}</option>)}</select></label>
        <dl>{series.map((s) => <div key={s.key}><dt>{s.label}</dt><dd>{amount(selected[s.key])}</dd></div>)}</dl>
      </div>
      <details className="cashflow-details">
        <summary>Точные суммы по месяцам</summary>
        <div className="table-wrap"><table className="cases monthly-table"><caption>{historical ? "История денежного потока" : "Расчёт по месяцам"}, ₽</caption><thead><tr><th scope="col">Месяц</th>{series.map((s) => <th scope="col" key={s.key}>{s.label}</th>)}</tr></thead><tbody>{points.map((point) => <tr key={point.period}><th scope="row">{periodLabel(point.period)}</th>{series.map((s) => <td key={s.key}>{amount(point[s.key])}</td>)}</tr>)}</tbody></table></div>
      </details>
    </div>
  );
}

function ReserveNote({ rows, scenario }: { rows: CashMonth[]; scenario: string }) {
  const first = firstCashShortfall(rows);
  if (!first) return null;
  const minimum = Math.min(...rows.map((row) => row.closing_cash_kopecks));
  return (
    <aside className={`cashflow-reserve${minimum < 0 ? " cashflow-reserve-low" : ""}`}>
      <strong>{scenario} · {fullPeriod(first.period)}: понадобится резерв {amount(first.shortfall)}</strong>
      <p>Поток — {amount(first.cfads_kopecks)}, все кредитные платежи — {amount(first.existing_debt_payment_kopecks + first.new_debt_payment_kopecks)}.</p>
      <p>{minimum >= 0 ? "По расчёту денежного резерва хватает." : "По расчёту денег не хватает даже с учётом резерва."} Минимальный остаток после платежей за показанный период — {amount(minimum)}.</p>
    </aside>
  );
}

export function AssessmentCashFlow({ financial, option, requestedAmount }: { financial: FinancialSnapshot; option?: Candidate; requestedAmount?: number }) {
  const base = option?.base ?? financial.base ?? [];
  const stress = option?.stress ?? financial.stress ?? [];
  const points = cashFlowPoints(base, stress);
  const percent = (bps: number) => `${(bps / 100).toLocaleString("ru-RU", { maximumFractionDigits: 1 })}%`;
  return (
    <div className="assessment-cashflow">
      <dl className="cashflow-facts">
        <div><dt>Доля потока в поступлениях</dt><dd>{percent(financial.operating_margin_bps)}</dd><p>После операционных расходов, налогов и изъятий, до кредитных платежей.</p></div>
        {financial.largest_customer_share_bps !== null && financial.largest_customer_share_bps !== undefined && <div><dt>Крупнейший покупатель</dt><dd>{percent(financial.largest_customer_share_bps)}</dd><p>Доля поступлений от одного клиента. Чем она меньше, тем ниже зависимость.</p></div>}
      </dl>
      <div className="cashflow-heading">
        <h3>{option ? `Поток и платежи по варианту ${amount(option.schedule.amount_kopecks)}` : "Денежный поток без нового кредита"}</h3>
        {option && requestedAmount !== undefined && <p className="muted">{option.uncovered_need_kopecks > 0 ? `До запрошенной суммы не хватает ${amount(option.uncovered_need_kopecks)}.` : "Рассчитанный вариант покрывает запрошенную сумму полностью."}</p>}
        <p className="field-hint">Денежный поток — деньги для обслуживания долга. Платежи включают действующие кредиты{option ? " и выбранный новый кредит" : ""}.</p>
      </div>
      <CashFlowTimeline points={points} />
      <ReserveNote rows={base} scenario="Базовый сценарий" />
      <ReserveNote rows={stress} scenario="Стресс-сценарий" />
      {option && <div className="cashflow-coverage">
        <h3>Покрытие всех кредитных платежей</h3>
        <CoverageBar label="Базовый сценарий" bps={option.base_coverage_bps} threshold={1.3} />
        <CoverageBar label="Стресс-сценарий" bps={option.stress_coverage_bps} threshold={1.1} />
        <p className="field-hint">{option.schedule.term_months <= 12 ? "Отношение суммы потока к сумме платежей за весь срок кредита." : "Минимальное покрытие среди всех последовательных периодов по 12 месяцев."} Покрытие за период не исключает нехватки поступлений в отдельном месяце. Чёрная отметка — минимум банка, шкала — от 0 до ×3.</p>
      </div>}
      <details className="cashflow-details">
        <summary>Допущения и денежный резерв</summary>
        <p>Базовый сценарий повторяет соответствующие месяцы прошлого года без роста.</p>
        <p>Стресс: выручка ниже на 20%, переменные затраты ниже на 10%; 20% сниженных поступлений приходят на месяц позже. В первом месяце ещё нет отложенных поступлений от предыдущего месяца.</p>
        <p>Начальный денежный резерв — {amount(financial.initial_cash_kopecks)}. Остаток учитывает резерв, накопленный поток и все кредитные платежи.</p>
      </details>
      {(financial.history?.length ?? 0) > 0 && <details className="cashflow-details">
        <summary>История потока · {financial.history!.length} месяцев</summary>
        <p className="field-hint">Период данных: {financial.period_from} — {financial.period_to}. Кредитные платежи за прошлые месяцы здесь не представлены.</p>
        <CashFlowTimeline points={historicalCashFlowPoints(financial.history!)} historical />
      </details>}
      <p className="field-hint">Сведения в демо синтетические. Сценарии описывают допущения расчёта, а не гарантируют будущие поступления.</p>
    </div>
  );
}
