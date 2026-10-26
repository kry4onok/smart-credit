import { useCallback, useEffect, useState } from "react";
import { assessmentApi } from "../../api/http";
import { ApiError, call } from "../../api/problem";
import type { AssessmentFactor, AssessmentResult, BankAssessment, BankCase, Candidate } from "../../api/types";
import { CashFlowChart, CoverageBar } from "../../components/CashFlowChart";
import { Badge, ErrorNote, useAction } from "../../components/ui";
import { dateTime, money, months, rate, rub, shortId } from "../../lib/format";
import {
  assessmentStatusLabel,
  checkCodeLabel,
  conclusionLabel,
  freshnessLabel,
  sourceKindLabel,
  sourceStatusLabel,
  subjectRoleLabel,
  v2FactorLabel,
} from "../../lib/labels";
import { etag, useIntent, usePolling } from "../../lib/util";

const failureLabel: Record<string, string> = {
  BASE_CASH_GAP: "На запрошенных условиях денежного потока не хватает в базовом сценарии",
  STRESS_CASH_GAP: "На запрошенных условиях денег не хватает при падении выручки",
  COVERAGE_BELOW_MINIMUM: "Покрытие платежа ниже ×1,3 / ×1,1",
  PREFERRED_PAYMENT_EXCEEDED: "Платёж выше желаемого клиентом",
  PREFERRED_TERM_EXCEEDED: "Срок длиннее желаемого клиентом",
  PRODUCT_RANGE: "Условия вне диапазона продукта",
  BELOW_USEFUL_AMOUNT: "Доступная сумма ниже минимально полезной",
};

function factorValue(f: AssessmentFactor): string {
  if (f.unit === "DAYS") return `${f.value} дн. просрочки`;
  if (f.unit === "BASIS_POINTS") return `${(f.value / 100).toLocaleString("ru-RU", { maximumFractionDigits: 1 })}%`;
  return String(f.value);
}

function Factors({ result }: { result: AssessmentResult }) {
  const r = result.risk;
  return (
    <div className="v2-risk">
      <div className="score-head">
        {r.grade && <span className={`grade grade-${r.grade}`}>{r.grade}</span>}
        <div>
          {r.score !== undefined && r.score !== null ? (
            <>
              <strong className="score-value">{r.score}</strong>
              <span className="muted"> из 1000 по скоркарте денежного потока</span>
            </>
          ) : (
            <span className="muted">Балл не рассчитан: {r.status === "NOT_RUN" ? "допуск не пройден" : r.status}</span>
          )}
          <div className="score-meta">{r.model_version}</div>
        </div>
      </div>
      <ul className="v2-factors">
        {(r.factors ?? []).map((f) => (
          <li key={f.factor_id}>
            <div className="v2-factor-head">
              <strong>{v2FactorLabel[f.factor_id] ?? f.factor_id}</strong>
              <span>{factorValue(f)}</span>
            </div>
            <span className="v2-factor-bar" aria-label={`${f.contribution} из ${f.max_points} баллов`}>
              <span style={{ width: `${f.max_points ? (f.contribution / f.max_points) * 100 : 0}%` }} />
            </span>
            <small>
              {f.contribution} из {f.max_points} баллов, вес {Math.round(f.weight_bps / 100)}%
            </small>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Checks({ result }: { result: AssessmentResult }) {
  const c = result.checks;
  return (
    <div className="v2-checks">
      <p>
        Допуск: <strong className={`conclusion conclusion-${c.conclusion}`}>{conclusionLabel[c.conclusion] ?? c.conclusion}</strong>
        {(c.codes ?? []).map((code) => (
          <span key={code} className="rule">
            {checkCodeLabel[code] ?? code} <code>{code}</code>
          </span>
        ))}
      </p>
      <div className="table-wrap">
        <table className="cases sources">
          <thead>
            <tr>
              <th>Источник</th>
              <th>Лицо</th>
              <th>Ответ</th>
              <th>Вывод</th>
            </tr>
          </thead>
          <tbody>
            {(c.sources ?? []).map((s) => (
              <tr key={s.id}>
                <td>
                  {sourceKindLabel[s.kind] ?? s.kind}
                  {s.provenance.is_mock && <em className="tag">мок</em>}
                </td>
                <td>{subjectRoleLabel[s.role] ?? s.role}</td>
                <td>{sourceStatusLabel[s.status] ?? s.status}</td>
                <td className={`conclusion conclusion-${s.conclusion}`}>{conclusionLabel[s.conclusion] ?? s.conclusion}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {c.registration_date_is_activity_proxy && <p className="field-hint">Начало деятельности оценено по дате регистрации.</p>}
    </div>
  );
}

function OptionRow({ c, selected, onSelect, onUse, canUse, requested }: { c: Candidate; selected: boolean; onSelect: () => void; onUse: () => void; canUse: boolean; requested: number }) {
  const s = c.schedule;
  return (
    <li className={`v2-option${selected ? " on" : ""}`}>
      <button type="button" className="option-select" aria-pressed={selected} onClick={onSelect}>
        <strong className="amount">{money(rub(s.amount_kopecks))}</strong>
        <span>
          {months(s.term_months)}, {rate(s.rate_bps)}, платёж {money(rub(s.payment_kopecks))}
        </span>
        <span className="muted">
          {c.product_name}
          {c.uncovered_need_kopecks > 0 ? `, не хватает ${money(rub(c.uncovered_need_kopecks))} из ${money(rub(requested))}` : ""}
        </span>
      </button>
      <CoverageBar label="База" bps={c.base_coverage_bps} threshold={1.3} />
      <CoverageBar label="Стресс" bps={c.stress_coverage_bps} threshold={1.1} />
      <button type="button" className="btn btn-secondary btn-sm" disabled={!canUse} onClick={onUse} title={canUse ? undefined : "Вариант можно предложить по актуальной оценке, когда дело у вас в работе"}>
        Предложить этот вариант
      </button>
    </li>
  );
}

// OfferBasis is what the bank accepts as an offer: only a candidate of the current assessment, with
// its exact product, amount, term and rate. null means the case has no assessment (legacy flow).
export type OfferBasis = { candidates: Candidate[]; stale: boolean } | null;

export function offerBasis(data: BankAssessment | null): OfferBasis {
  if (!data) return null;
  const fresh = data.freshness === "CURRENT";
  const now = Date.now();
  return { stale: !fresh, candidates: fresh ? (data.assessment?.candidates ?? []).filter((c) => Date.parse(c.valid_until) > now) : [] };
}

// The assessment is still being calculated only while the case is in the automatic pipeline.
const inPipeline = new Set<string>(["SUBMITTED", "VALIDATION", "ENRICHMENT", "SCORING", "DECISION"]);

// BankAssessmentPanel shows the cash-flow assessment v2 to the analyst: admission checks, scorecard
// factors, the cash-flow forecast and the calculated options that can be confirmed as an offer.
export function BankAssessmentPanel({ kase, onUse, onChanged, onBasis }: { kase: BankCase; onUse: (c: Candidate) => void; onChanged: () => void; onBasis: (b: OfferBasis) => void }) {
  const [data, setData] = useState<BankAssessment | null>(null);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [selected, setSelected] = useState(0);
  const [history, setHistory] = useState<AssessmentResult[] | null>(null);
  const [historyError, setHistoryError] = useState<unknown>(null);
  const recheck = useAction();
  const recheckIntent = useIntent();

  const load = useCallback(async () => {
    try {
      const d = await call(assessmentApi.GET("/backoffice/applications/{application_id}/assessment", { params: { path: { application_id: kase.id } } }));
      setData(d);
      setMissing(false);
      setError(null);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) setMissing(true);
      else setError(e);
    }
  }, [kase.id]);

  useEffect(() => void load(), [load, kase.version, kase.audit_version]);
  useEffect(() => onBasis(missing ? null : offerBasis(data)), [data, missing]); // eslint-disable-line react-hooks/exhaustive-deps
  usePolling(load, 3000, data?.freshness === "PENDING");

  if (missing || (!data && !error)) return null;
  if (error) return <ErrorNote error={error} onRetry={load} />;
  const result = data!.assessment;
  const canDecide = kase.allowed_actions.includes("DECIDE");
  const canUse = canDecide && data!.freshness === "CURRENT";
  const canRecheck = canDecide && (kase.status === "MANUAL_REVIEW" || kase.status === "NEEDS_INFO");

  async function runRecheck() {
    await recheck.run(async () => {
      await recheckIntent.send((key) =>
        call(
          assessmentApi.POST("/backoffice/applications/{application_id}/recheck", {
            params: { path: { application_id: kase.id }, header: { "If-Match": etag(kase.version), "Idempotency-Key": key } },
          }),
        ),
      );
      onChanged();
      await load();
    });
  }

  async function loadHistory() {
    try {
      const h = await call(assessmentApi.GET("/backoffice/applications/{application_id}/assessments", { params: { path: { application_id: kase.id } } }));
      setHistory(h.items);
      setHistoryError(null);
    } catch (e) {
      setHistoryError(e);
    }
  }

  const candidates = result?.candidates ?? [];
  const option = candidates[Math.min(selected, candidates.length - 1)];
  const baseline = data!.baseline_v1?.response as { grade?: string; score?: number } | undefined;

  return (
    <section className="panel v2" id="assessment" aria-labelledby="v2-title">
      <div className="assess-head">
        <div>
          <h2 id="v2-title">Оценка денежного потока</h2>
          <p className="muted">
            Поколение {data!.assessment_generation}, ревизия данных {data!.input_revision}
            {result ? `, расчёт от ${dateTime(result.as_of)}` : ""}
          </p>
        </div>
        <span className="v2-badges">
          <Badge tone={data!.freshness === "CURRENT" ? "success" : data!.freshness === "PENDING" ? "progress" : "attention"}>{freshnessLabel[data!.freshness]}</Badge>
          {result && <Badge tone={result.status === "REJECT" ? "danger" : result.status === "READY_FOR_CONFIRMATION" ? "success" : "attention"}>{assessmentStatusLabel[result.status] ?? result.status}</Badge>}
        </span>
      </div>

      {!result &&
        (inPipeline.has(kase.status) ? (
          <p className="pending">
            <span className="spinner" aria-hidden />
            Оценка выполняется…
          </p>
        ) : (
          <p className="muted">Расчёта денежного потока по этому делу нет: продукт заявки не использует оценку v2 или дело закрыто до расчёта. Решение принимается по проверкам и скорингу ниже.</p>
        ))}

      {result && (
        <>
          {(result.original_failure_codes ?? []).length > 0 && (
            <p className="note note-attention">
              Запрошенные условия: {result.original_failure_codes!.map((c) => failureLabel[c] ?? c).join("; ")}.
            </p>
          )}
          {(result.issues ?? []).length > 0 && (
            <ul className="issues">
              {result.issues!.map((i) => (
                <li key={i.code + i.path}>
                  {i.message} <code>{i.code}</code>
                </li>
              ))}
            </ul>
          )}
          <div className="v2-grid">
            <Factors result={result} />
            <Checks result={result} />
          </div>
          {baseline && (
            <p className="field-hint">
              Прежняя модель v1 для сравнения: {baseline.grade?.replace("GRADE_", "")} {baseline.score !== undefined ? `(${baseline.score})` : ""}.
            </p>
          )}
          {result.financial && (
            <CashFlowChart
              caption={option ? `Поток и платежи по варианту ${money(rub(option.schedule.amount_kopecks))}` : "Денежный поток без нового кредита"}
              history={result.financial.history}
              base={option?.base ?? result.financial.base}
              stress={option?.stress ?? result.financial.stress}
            />
          )}
          {candidates.length > 0 && (
            <>
              <h3>Расчётные варианты</h3>
              <ul className="v2-options" id="options">
                {candidates.map((c, i) => (
                  <OptionRow
                    key={c.candidate_id}
                    c={c}
                    selected={i === selected}
                    onSelect={() => setSelected(i)}
                    onUse={() => onUse(c)}
                    canUse={canUse}
                    requested={kase.application.amount.amount_kopecks}
                  />
                ))}
              </ul>
              <p className="field-hint">Варианты действуют до {dateTime(result.valid_until)}. Банк примет только ставку и условия из актуального расчёта.</p>
            </>
          )}
        </>
      )}

      <ErrorNote error={recheck.error} />
      <div className="form-actions">
        {canRecheck && (
          <button type="button" className="btn btn-secondary btn-sm" disabled={recheck.busy} onClick={() => void runRecheck()}>
            {recheck.busy ? "Запускаем…" : "Пересчитать оценку"}
          </button>
        )}
        <button type="button" className="btn btn-quiet btn-sm" onClick={() => void loadHistory()}>
          История оценок
        </button>
      </div>
      <ErrorNote error={historyError} onRetry={() => void loadHistory()} />
      {history && (
        <ol className="audit">
          {history.map((h) => (
            <li key={h.assessment_id}>
              <time dateTime={h.as_of}>{dateTime(h.as_of)}</time>
              <div>
                <strong>
                  Поколение {h.assessment_generation}: {assessmentStatusLabel[h.status] ?? h.status}
                </strong>
                <p>
                  {h.risk.grade ? `Категория ${h.risk.grade}, ${h.risk.score}. ` : ""}
                  Вариантов: {(h.candidates ?? []).length}. Оценка {shortId(h.assessment_id)}.
                </p>
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
