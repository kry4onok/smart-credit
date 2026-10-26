import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { assessmentApi } from "../../api/http";
import { ApiError, call } from "../../api/problem";
import type { Candidate, ClientApplication, ClientAssessment, PublicResult } from "../../api/types";
import { CashFlowChart, CoverageBar } from "../../components/CashFlowChart";
import { CompareBars } from "../../components/Compare";
import { HelpPanel, clientModes } from "../../components/HelpPanel";
import { Badge, ErrorNote, useNow } from "../../components/ui";
import { dateTime, money, months, rate, rub } from "../../lib/format";
import { assessmentStatusLabel } from "../../lib/labels";

// Impact of one parameter on the bank's view, from −2 (hurts a lot) to +2 (helps a lot).
type Impact = { id: string; label: string; value: string; impact: -2 | -1 | 0 | 1 | 2; text: string };

const impactWord: Record<number, string> = { 2: "сильно помогает", 1: "помогает", 0: "нейтрально", [-1]: "снижает шансы", [-2]: "сильно мешает" };

const pct = (bps: number) => `${Math.round(bps / 100)}%`;

// Bands follow the published expert scorecard steps (docs/ASSESSMENT_V2.md): the client sees which
// of their own figures moved the result up or down, without the bank's internal points.
export function impacts(result: PublicResult, requestedKopecks: number | undefined, option: Candidate | undefined): Impact[] {
  const out: Impact[] = [];
  const f = result.financial;
  if (f) {
    const m = f.operating_margin_bps;
    out.push({
      id: "margin",
      label: "Запас денежного потока",
      value: pct(m),
      impact: m >= 2000 ? 2 : m >= 1500 ? 1 : m >= 1000 ? 0 : m >= 0 ? -1 : -2,
      text: `После всех расходов, налогов и изъятий у бизнеса остаётся ${pct(m)} поступлений.${m < 1000 ? " Чем выше этот запас, тем больше кредит может выдержать бизнес." : ""}`,
    });
    const c = f.largest_customer_share_bps;
    if (c !== null && c !== undefined) {
      out.push({
        id: "concentration",
        label: "Зависимость от одного покупателя",
        value: pct(c),
        impact: c <= 2000 ? 2 : c <= 3500 ? 1 : c <= 5000 ? 0 : c <= 7500 ? -1 : -2,
        text: `Крупнейший покупатель приносит ${pct(c)} выручки.${c > 5000 ? " Если он уйдёт, бизнесу будет сложно платить по кредиту." : " Выручка распределена по нескольким клиентам."}`,
      });
    }
  }
  if (option) {
    const base = option.base_coverage_bps / 10000;
    const stress = option.stress_coverage_bps / 10000;
    out.push({
      id: "coverage",
      label: "Платёж к денежному потоку",
      value: `×${base.toLocaleString("ru-RU", { maximumFractionDigits: 1 })}`,
      impact: base >= 2 ? 2 : base >= 1.3 ? 1 : -2,
      text: `Поток бизнеса покрывает платёж в ${base.toLocaleString("ru-RU", { maximumFractionDigits: 1 })} раза, а при падении выручки на 20% — в ${stress.toLocaleString("ru-RU", { maximumFractionDigits: 1 })}. Банку нужно не меньше ×1,3 и ×1,1.`,
    });
    if (requestedKopecks && option.uncovered_need_kopecks > 0) {
      const share = 1 - option.uncovered_need_kopecks / requestedKopecks;
      out.push({
        id: "amount",
        label: "Запрошенная сумма",
        value: pct(Math.round(share * 10000)),
        impact: share >= 0.8 ? -1 : -2,
        text: `Денежного потока хватает на ${money(rub(option.schedule.amount_kopecks))} из ${money(rub(requestedKopecks))}. Не хватает ${money(rub(option.uncovered_need_kopecks))}.`,
      });
    } else if (requestedKopecks) {
      out.push({ id: "amount", label: "Запрошенная сумма", value: "100%", impact: 1, text: "Бизнес может обслуживать кредит на всю запрошенную сумму." });
    }
  }
  return out;
}

function ImpactChart({ items }: { items: Impact[] }) {
  if (!items.length) return null;
  return (
    <div className="impact" id="factors">
      <div className="impact-axis" aria-hidden>
        <span>мешает</span>
        <span>помогает</span>
      </div>
      <ul>
        {items.map((i) => (
          <li key={i.id} className={`impact-row impact-${i.impact > 0 ? "pos" : i.impact < 0 ? "neg" : "zero"}`}>
            <div className="impact-label">
              <strong>{i.label}</strong>
              <span className="impact-value">{i.value}</span>
            </div>
            <span className="impact-bar" aria-hidden>
              <span className="impact-mid" />
              <span
                className="impact-fill"
                style={i.impact >= 0 ? { left: "50%", width: `${(i.impact / 2) * 50}%` } : { right: "50%", width: `${(-i.impact / 2) * 50}%` }}
              />
            </span>
            <p>
              <span className="impact-word">{impactWord[i.impact]}.</span> {i.text}
            </p>
          </li>
        ))}
      </ul>
    </div>
  );
}

function OptionCard({ c, selected, onSelect, requested }: { c: Candidate; selected: boolean; onSelect: () => void; requested?: { amount: number; term: number } }) {
  const s = c.schedule;
  return (
    <article className={`option${selected ? " option-on" : ""}`}>
      <button type="button" className="option-select" aria-pressed={selected} onClick={onSelect}>
        <span className="option-amount amount">{money(rub(s.amount_kopecks))}</span>
        <span>
          {months(s.term_months)} под {rate(s.rate_bps)}
        </span>
      </button>
      <dl className="option-terms">
        <div>
          <dt>Платёж в месяц</dt>
          <dd>{money(rub(s.payment_kopecks))}</dd>
        </div>
        <div>
          <dt>Переплата</dt>
          <dd>{money(rub(s.total_interest_kopecks))}</dd>
        </div>
      </dl>
      <CoverageBar label="Запас по потоку" bps={c.base_coverage_bps} threshold={1.3} />
      <CoverageBar label="Если выручка упадёт" bps={c.stress_coverage_bps} threshold={1.1} />
      {requested && c.uncovered_need_kopecks > 0 && <p className="option-gap">Не хватит до запроса: {money(rub(c.uncovered_need_kopecks))}</p>}
      {requested && selected && (
        <CompareBars
          rows={[
            { label: "Сумма", requested: requested.amount, offered: s.amount_kopecks, format: (n) => money(rub(n)) },
            { label: "Срок", requested: requested.term, offered: s.term_months, format: months },
          ]}
        />
      )}
    </article>
  );
}

// Statuses before the bank's decision: only then the calculated options are still a forecast of
// what the manager may confirm. After an offer, a refusal or a cancellation they would mislead.
const underReview = new Set<ClientApplication["status"]>(["SUBMITTED", "UNDER_REVIEW", "NEEDS_INFO"]);

// AssessmentView explains the bank's cash-flow assessment to the client after submission.
export function AssessmentView({ app }: { app: ClientApplication }) {
  const [data, setData] = useState<ClientAssessment | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [selected, setSelected] = useState(0);
  const navigate = useNavigate();
  const now = useNow();

  useEffect(() => {
    call(assessmentApi.GET("/client/applications/{application_id}/assessment", { params: { path: { application_id: app.id } } }))
      .then((d) => {
        setData(d);
        setError(null);
      })
      .catch((e) => {
        // Products without the cash-flow assessment have no such resource.
        if (e instanceof ApiError && (e.status === 404 || e.status === 501)) setData(null);
        else setError(e);
      });
  }, [app.id, app.version]);

  if (error) return <ErrorNote error={error} />;
  if (!data) return null;
  const result = data.assessment;
  const decided = !underReview.has(app.status);
  if (!result) {
    if (app.status === "DRAFT" || decided) return null;
    return (
      <section className="panel assess">
        <h2>Оценка банка</h2>
        <p className="pending">
          <span className="spinner" aria-hidden />
          Банк собирает данные из источников и считает денежный поток. Результат появится здесь.
        </p>
      </section>
    );
  }

  const f = result.financial;
  // An outdated calculation keeps the charts for context, but its options are no longer valid.
  const stale = data.freshness === "STALE" || Date.parse(result.valid_until) <= now;
  const help = (
    <HelpPanel
      applicationId={app.id}
      audience="client"
      modes={!decided && !stale && result.candidates?.length ? clientModes : clientModes.filter((m) => m.mode !== "COMPARE_OPTIONS")}
      onOpenProfile={() => navigate("/cabinet/company")}
    />
  );

  // After the decision the offer or the refusal above is what counts; the calculation stays as a
  // collapsed reference without the options and the pre-decision status.
  if (decided) {
    return (
      <>
        {f && (
          <details className="panel summary">
            <summary>Расчёт денежного потока</summary>
            <p className="muted">Расчёт от {dateTime(result.as_of)}, показан для справки.</p>
            <CashFlowChart caption="Денежный поток бизнеса" history={f.history} base={f.base} stress={f.stress} />
          </details>
        )}
        {help}
      </>
    );
  }

  const candidates = stale ? [] : (result.candidates ?? []);
  const option = candidates[Math.min(selected, candidates.length - 1)];
  const requested = app.form ? { amount: app.form.amount.amount_kopecks, term: app.form.term_months } : undefined;
  const tone = stale ? "muted" : result.status === "READY_FOR_CONFIRMATION" ? "success" : result.status === "REJECT" ? "danger" : "attention";

  return (
    <>
      <section className="panel assess" id="assessment" aria-labelledby="assess-title">
        <div className="assess-head">
          <div>
            <h2 id="assess-title">Как банк оценил заявку</h2>
            <p className="muted">
              Расчёт от {dateTime(result.as_of)}
              {data.freshness === "PENDING" && !stale && " — данные обновляются"}
            </p>
          </div>
          <Badge tone={tone}>{stale ? "Расчёт устарел" : (assessmentStatusLabel[result.status] ?? result.status)}</Badge>
        </div>

        {stale && <p className="field-hint">Расчёт устарел: банк пересчитает его, когда получит новые данные. Варианты кредита появятся после пересчёта.</p>}

        <ImpactChart items={impacts(result, requested?.amount, option)} />

        {(result.issues ?? []).length > 0 && (
          <ul className="issues">
            {result.issues!.map((i) => (
              <li key={i.code + i.path}>{i.message}</li>
            ))}
          </ul>
        )}

        {f && (
          <CashFlowChart
            caption={option ? `Денежный поток и платежи по варианту ${money(rub(option.schedule.amount_kopecks))}` : "Денежный поток бизнеса"}
            history={f.history}
            base={option?.base ?? f.base}
            stress={option?.stress ?? f.stress}
          />
        )}
        <p className="field-hint">
          Факт — по выпискам за {f ? `${f.period_from} — ${f.period_to}` : "последние 12 месяцев"}. Прогноз повторяет прошлый год без роста; стресс — выручка ниже
          на 20%. Сведения в демо синтетические.
        </p>
      </section>

      {candidates.length > 0 && (
        <section className="panel" id="options" aria-labelledby="options-title">
          <h2 id="options-title">Варианты, которые рассчитал банк</h2>
          <p className="muted">
            Это расчёт, а не оферта: менеджер банка подтвердит один из вариантов, и он придёт вам предложением. Действует до {dateTime(result.valid_until)}.
          </p>
          <div className="options">
            {candidates.map((c, i) => (
              <OptionCard key={c.candidate_id} c={c} selected={i === selected} onSelect={() => setSelected(i)} requested={requested} />
            ))}
          </div>
        </section>
      )}

      {help}
    </>
  );
}
