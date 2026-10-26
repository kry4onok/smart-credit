import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { assessmentApi } from "../../api/http";
import { ApiError, call } from "../../api/problem";
import type { Candidate, ClientApplication, ClientAssessment } from "../../api/types";
import { CoverageBar } from "../../components/CashFlowChart";
import { AssessmentCashFlow } from "../../components/AssessmentCashFlow";
import { CompareBars } from "../../components/Compare";
import { HelpPanel, clientModes } from "../../components/HelpPanel";
import { Badge, ErrorNote, useNow } from "../../components/ui";
import { dateTime, money, months, rate, rub } from "../../lib/format";
import { assessmentStatusLabel } from "../../lib/labels";

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
      <CoverageBar label="Базовое покрытие" bps={c.base_coverage_bps} threshold={1.3} />
      <CoverageBar label="Покрытие в стрессе" bps={c.stress_coverage_bps} threshold={1.1} />
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
            <AssessmentCashFlow financial={f} />
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
            <h2 id="assess-title">Денежный поток и кредитная нагрузка</h2>
            <p className="muted">
              Расчёт от {dateTime(result.as_of)}
              {data.freshness === "PENDING" && !stale && " — данные обновляются"}
            </p>
          </div>
          <Badge tone={tone}>{stale ? "Расчёт устарел" : (assessmentStatusLabel[result.status] ?? result.status)}</Badge>
        </div>

        {stale && <p className="field-hint">Расчёт устарел: банк пересчитает его, когда получит новые данные. Варианты кредита появятся после пересчёта.</p>}

        {(result.issues ?? []).length > 0 && (
          <ul className="issues">
            {result.issues!.map((i) => (
              <li key={i.code + i.path}>{i.message}</li>
            ))}
          </ul>
        )}

        {f && <AssessmentCashFlow financial={f} option={option} requestedAmount={requested?.amount} />}
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
