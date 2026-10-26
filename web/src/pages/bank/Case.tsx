import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { bankApi, isMock } from "../../api/http";
import { ApiError, call } from "../../api/problem";
import type { AnalystDecisionRequest, AuditEntry, BankCase, Candidate, CaseSummary, DocumentType, InfoField, Product } from "../../api/types";
import { CompareBars, offerRows } from "../../components/Compare";
import { MoneyInput } from "../../components/inputs";
import { HelpPanel, analystModes } from "../../components/HelpPanel";
import { BankAssessmentPanel, type OfferBasis } from "./AssessmentPanel";
import { BankPipeline } from "../../components/Pipeline";
import { Badge, Choices, Dl, ErrorNote, Field, Loading, useAction, useTitle } from "../../components/ui";
import { date, dateTime, money, months, rate, rub, shortId } from "../../lib/format";
import {
  actorLabel,
  auditActionLabel,
  bankStatusLabel,
  bankStatusTone,
  companyStatusLabel,
  documentLabel,
  infoFieldLabel,
  purposeLabel,
  recommendationLabel,
  ruleLabel,
} from "../../lib/labels";
import { appendPage, etag, useIntent, usePolling } from "../../lib/util";
import { ScorePanel } from "./Score";

const automatic = new Set(["SUBMITTED", "VALIDATION", "ENRICHMENT", "SCORING", "DECISION", "OFFER_ACCEPTED", "SIGNING"]);

export function CasePage() {
  const { id = "" } = useParams();
  const [kase, setCase] = useState<BankCase | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [auditKey, setAuditKey] = useState(0);
  const [prefill, setPrefill] = useState<Prefill | null>(null);
  const [basis, setBasis] = useState<OfferBasis>(null);

  const load = useCallback(async () => {
    try {
      const next = await call(bankApi.GET("/backoffice/applications/{application_id}", { params: { path: { application_id: id } } }));
      setCase((prev) => {
        if (prev && prev.audit_version !== next.audit_version) setAuditKey((k) => k + 1);
        return next;
      });
      setError(null);
    } catch (e) {
      setError(e);
    }
  }, [id]);

  useEffect(() => {
    setCase(null);
    void load();
  }, [load]);
  usePolling(load, 4000, !!kase && automatic.has(kase.status));
  useTitle(kase ? kase.company.name : "Дело");

  if (error && !kase) return <ErrorNote error={error} onRetry={load} />;
  if (!kase) return <Loading />;

  const updated = (next: BankCase) => {
    setCase(next);
    setAuditKey((k) => k + 1);
  };
  const amountToRevenue = kase.company.revenue_12m && kase.company.revenue_12m.amount_kopecks > 0 ? (kase.application.amount.amount_kopecks / kase.company.revenue_12m.amount_kopecks) * 100 : null;

  return (
    <>
      <Link to="/bank" className="back">
        Очередь
      </Link>
      <section className="case-head">
        <div>
          <h1>{kase.company.name}</h1>
          <p className="muted">
            ИНН {kase.company.inn}, дело № {shortId(kase.id)}, получено {dateTime(kase.created_at)}
          </p>
        </div>
        <span aria-live="polite">
          <Badge tone={bankStatusTone[kase.status]}>{bankStatusLabel[kase.status]}</Badge>
        </span>
      </section>
      <BankPipeline status={kase.status} />
      <ErrorNote error={error} />

      <div className="case-grid">
        <div className="case-main">
          <section className="panel">
            <h2>Запрос</h2>
            <div className="request-line">
              <strong className="amount">{money(kase.application.amount)}</strong>
              <span>
                на {months(kase.application.term_months)}, {purposeLabel[kase.application.purpose].toLowerCase()}
              </span>
            </div>
            <Dl
              items={[
                ["Продукт", `${kase.product_name}, версия ${kase.product_version}`],
                ["Документы", kase.application.document_types.length ? kase.application.document_types.map((d) => documentLabel[d]).join(", ") : "не приложены"],
                ["Сумма к выручке", amountToRevenue === null ? "выручка не указана" : `${amountToRevenue.toFixed(1)}%`],
              ]}
            />
          </section>

          <BankAssessmentPanel kase={kase} onUse={(c) => setPrefill({ candidate: c, nonce: Date.now() })} onChanged={() => void load()} onBasis={setBasis} />

          <AssistantSummary id={kase.id} />
          {/* The in-browser mock has no assistant: the panel only makes sense against the backend. */}
          {!isMock && <HelpPanel applicationId={kase.id} audience="analyst" modes={analystModes} />}

          <section className="panel">
            <h2>Скоринг</h2>
            <ScorePanel score={kase.score} />
            {(kase.recommendation || (kase.triggered_rules && kase.triggered_rules.length > 0)) && (
              <div className="rules">
                {kase.recommendation && (
                  <p>
                    Рекомендация правил: <strong>{recommendationLabel[kase.recommendation]}</strong>
                  </p>
                )}
                {kase.triggered_rules && kase.triggered_rules.length > 0 && (
                  <p>
                    Сработали правила:{" "}
                    {kase.triggered_rules.map((r) => (
                      <span key={r} className="rule">
                        {ruleLabel[r] ?? r} <code>{r}</code>
                      </span>
                    ))}
                  </p>
                )}
              </div>
            )}
          </section>

          <section className="panel two-col">
            <div>
              <h2>Проверки</h2>
              {kase.checks ? (
                <>
                  {kase.checks.bureau_error && <p className="note note-danger">Источник вернул ошибку: {kase.checks.bureau_error}</p>}
                  <Dl
                    items={[
                      ["Статус компании (ЕГРЮЛ)", <StatusValue key="s" value={kase.checks.company_status} />],
                      ["Дата регистрации", date(kase.checks.registered_at)],
                      ["ОКВЭД", kase.checks.okved ?? "—"],
                      ["Долг по БКИ", money(kase.checks.total_debt)],
                      ["Просрочек за 12 мес.", String(kase.checks.overdue_count_12m)],
                      ["Макс. просрочка", `${kase.checks.max_overdue_days} дн.`],
                    ]}
                  />
                </>
              ) : (
                <p className="muted">Внешние проверки ещё не выполнены.</p>
              )}
            </div>
            <div>
              <h2>Анкета клиента</h2>
              <Dl
                items={[
                  ["ОКВЭД", kase.company.okved],
                  ["Зарегистрирована", date(kase.company.registered_at)],
                  ["Выручка 12 мес.", money(kase.company.revenue_12m)],
                  ["Прибыль 12 мес.", money(kase.company.profit_12m)],
                  ["Заявленные обязательства", money(kase.company.obligations)],
                ]}
              />
            </div>
          </section>

          {(kase.info_request || kase.offer || kase.public_reason) && (
            <section className="panel">
              <h2>Что видит клиент</h2>
              {kase.info_request && (
                <div className="sub">
                  <h3>Открытый запрос данных</h3>
                  <p>{kase.info_request.message}</p>
                  <p className="muted">
                    {[...kase.info_request.required_fields.map((f) => infoFieldLabel[f]), ...kase.info_request.required_document_types.map((d) => documentLabel[d])].join(", ")}
                  </p>
                </div>
              )}
              {kase.offer && (
                <div className="sub">
                  <h3>{kase.offer.kind === "COUNTER_OFFER" ? "Контрпредложение" : "Оферта"}</h3>
                  <p>
                    {kase.offer.product_name}: {money(kase.offer.amount)} на {months(kase.offer.term_months)} под {rate(kase.offer.rate_bps)}, до{" "}
                    {dateTime(kase.offer.expires_at)}
                  </p>
                  <CompareBars rows={offerRows(kase.application, kase.offer)} />
                  {kase.offer.explanation && <p className="muted">{kase.offer.explanation}</p>}
                </div>
              )}
              {kase.public_reason && kase.public_reason !== kase.offer?.explanation && (
                <div className="sub">
                  <h3>Опубликованное объяснение</h3>
                  <p>{kase.public_reason}</p>
                </div>
              )}
            </section>
          )}

          <Audit id={kase.id} refreshKey={auditKey} />
        </div>

        <aside className="case-side">
          <Actions kase={kase} onUpdated={updated} reload={load} prefill={prefill} basis={basis} />
        </aside>
      </div>
    </>
  );
}

function StatusValue({ value }: { value: string }) {
  const bad = value === "LIQUIDATING" || value === "LIQUIDATED";
  return <span className={bad ? "text-danger" : undefined}>{companyStatusLabel[value] ?? value}</span>;
}

function AssistantSummary({ id }: { id: string }) {
  const [summary, setSummary] = useState<CaseSummary | null>(null);
  const act = useAction();
  async function generate() {
    const result = await act.run(() => call(bankApi.POST("/backoffice/applications/{application_id}/summary", { params: { path: { application_id: id } } })));
    if (result) setSummary(result);
  }
  return (
    <section className="panel assistant">
      <div className="assistant-head">
        <h2>Резюме дела</h2>
        <button type="button" className="btn btn-secondary btn-sm" disabled={act.busy} onClick={() => void generate()}>
          {act.busy ? "Готовим…" : summary ? "Обновить" : "Сформировать"}
        </button>
      </div>
      {!summary && !act.busy && <p className="muted">Короткий пересказ дела от локальной модели. Не влияет на решение и score.</p>}
      <ErrorNote error={act.error} />
      {summary && (
        <>
          <p className="assistant-text">{summary.text}</p>
          <p className="assistant-source">{summary.source === "assistant" ? "Подготовлено моделью с открытыми весами" : "Шаблонный текст: модель недоступна или не ответила вовремя"}</p>
        </>
      )}
    </section>
  );
}

function Audit({ id, refreshKey }: { id: string; refreshKey: number }) {
  const [items, setItems] = useState<AuditEntry[] | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const shown = useRef(0);
  const pager = useAction();

  const page = useCallback(
    (after?: string) =>
      call(bankApi.GET("/backoffice/applications/{application_id}/audit", { params: { path: { application_id: id }, query: { limit: 100, ...(after ? { cursor: after } : {}) } } })),
    [id],
  );

  // The history runs oldest to newest, so new entries land on the last page:
  // a refresh walks as many pages as are already on screen.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      let p = await page();
      let all = p.items;
      while (p.next_cursor && all.length < shown.current) {
        p = await page(p.next_cursor);
        all = appendPage(all, p.items);
      }
      if (cancelled) return;
      shown.current = all.length;
      setItems(all);
      setCursor(p.next_cursor ?? null);
      setError(null);
    })().catch((e) => !cancelled && setError(e));
    return () => {
      cancelled = true;
    };
  }, [page, refreshKey]);

  function more() {
    if (!cursor || pager.busy) return;
    void pager.run(async () => {
      const p = await page(cursor);
      setItems((prev) => {
        const next = appendPage(prev, p.items);
        shown.current = next.length;
        return next;
      });
      setCursor(p.next_cursor ?? null);
    });
  }

  return (
    <section className="panel">
      <h2>История дела</h2>
      <ErrorNote error={error} />
      {items && items.length === 0 && <p className="muted">Записей пока нет.</p>}
      {items && (
        <ol className="audit">
          {items.map((e) => (
            <li key={e.id}>
              <time dateTime={e.occurred_at}>{dateTime(e.occurred_at)}</time>
              <div>
                <strong>
                  {e.from_status && e.to_status
                    ? `${bankStatusLabel[e.from_status]} → ${bankStatusLabel[e.to_status]}`
                    : (auditActionLabel[e.action] ?? (e.to_status ? bankStatusLabel[e.to_status] : e.action))}
                </strong>
                <p>{e.reason}</p>
                <small>
                  {actorLabel[e.actor_type]}
                  {e.actor_id ? ` ${shortId(e.actor_id)}` : ""}, {e.action}
                </small>
              </div>
            </li>
          ))}
        </ol>
      )}
      <ErrorNote error={pager.error} onRetry={more} />
      {cursor && (
        <button type="button" className="btn btn-quiet btn-sm" disabled={pager.busy} onClick={more}>
          {pager.busy ? "Загружаем…" : "Показать ещё"}
        </button>
      )}
    </section>
  );
}

type Prefill = { candidate: Candidate; nonce: number };

type FormProps = { kase: BankCase; onUpdated: (c: BankCase) => void; reload: () => Promise<void>; prefill: Prefill | null; basis: OfferBasis };

function Actions({ kase, onUpdated, reload, prefill, basis }: FormProps) {
  const take = useAction();
  const takeIntent = useIntent();
  const canTake = kase.allowed_actions.includes("TAKE");
  const canDecide = kase.allowed_actions.includes("DECIDE");

  async function takeCase() {
    const next = await take.run(() =>
      takeIntent.send((key) =>
        call(
          bankApi.POST("/backoffice/applications/{application_id}/take", {
            params: { path: { application_id: kase.id }, header: { "Idempotency-Key": key, "If-Match": etag(kase.version) } },
          }),
        ),
      ),
    );
    if (next) onUpdated(next);
    else void reload();
  }

  if (canDecide) return <DecisionForm kase={kase} onUpdated={onUpdated} reload={reload} prefill={prefill} basis={basis} />;
  return (
    <div className="panel side-panel">
      <h2>Действия</h2>
      {canTake ? (
        <>
          <p>Дело ждёт ручного решения. Возьмите его, чтобы другой аналитик не начал работу параллельно.</p>
          <ErrorNote error={take.error} />
          <button type="button" className="btn btn-primary btn-block" disabled={take.busy} onClick={() => void takeCase()}>
            {take.busy ? "Назначаем…" : "Взять в работу"}
          </button>
        </>
      ) : kase.status === "MANUAL_REVIEW" ? (
        <p className="muted">{kase.assignee_id ? `Дело в работе у аналитика ${shortId(kase.assignee_id)}.` : "Для этого дела вам доступен только просмотр."}</p>
      ) : automatic.has(kase.status) ? (
        <p className="pending">
          <span className="spinner" aria-hidden />
          Конвейер обрабатывает дело автоматически.
        </p>
      ) : (
        <p className="muted">Ручных действий на этом этапе нет.</p>
      )}
    </div>
  );
}

type Outcome = AnalystDecisionRequest["outcome"];

const outcomes: { value: Outcome; label: string; hint: string }[] = [
  { value: "APPROVE", label: "Одобрить", hint: "Оферта на запрошенных или близких условиях" },
  { value: "COUNTER", label: "Контрпредложение", hint: "Другие сумма, срок или продукт" },
  { value: "REQUEST_INFO", label: "Запросить данные", hint: "Клиент дополнит заявку" },
  { value: "REJECT", label: "Отказать", hint: "С безопасной причиной для клиента" },
];

const infoFields = Object.keys(infoFieldLabel) as InfoField[];
// The legacy request flow supports these types until the requirements engine is enabled.
const documentTypes: DocumentType[] = ["BANK_STATEMENT", "BUSINESS_PLAN"];

function parsePercent(text: string): number | null {
  const m = /^(\d{1,3})(?:[.,](\d{1,2}))?$/.exec(text.trim());
  return m ? Number(m[1]) * 100 + Number((m[2] ?? "").padEnd(2, "0")) : null;
}

function DecisionForm({ kase, onUpdated, reload, prefill, basis }: FormProps) {
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [products, setProducts] = useState<Product[] | null>(null);
  const [requirementIds, setRequirementIds] = useState<string[]>([]);
  const plan = kase.requirements_plan;
  const [productId, setProductId] = useState(kase.product_id);
  const [otherProduct, setOtherProduct] = useState("");
  const [amount, setAmount] = useState<number | null>(kase.application.amount.amount_kopecks);
  const [term, setTerm] = useState(String(kase.application.term_months));
  const [ratePct, setRatePct] = useState("");
  const [message, setMessage] = useState("");
  const [fields, setFields] = useState<InfoField[]>([]);
  const [docs, setDocs] = useState<DocumentType[]>([]);
  const [publicReason, setPublicReason] = useState("");
  const [comment, setComment] = useState("");
  const [shown, setShown] = useState(false);
  const [candidateId, setCandidateId] = useState<string | null>(null);
  const act = useAction();
  const decisionIntent = useIntent();

  useEffect(() => {
    call(bankApi.GET("/backoffice/products"))
      .then((r) => setProducts(r.items))
      .catch(() => setProducts(null));
  }, []);

  function applyCandidate(c: Candidate) {
    const s = c.schedule;
    const same = c.product_id === kase.product_id && s.amount_kopecks === kase.application.amount.amount_kopecks && s.term_months === kase.application.term_months;
    setCandidateId(c.candidate_id);
    setOutcome(same ? "APPROVE" : "COUNTER");
    setProductId(c.product_id);
    setAmount(s.amount_kopecks);
    setTerm(String(s.term_months));
    setRatePct((s.rate_bps / 100).toLocaleString("ru-RU", { maximumFractionDigits: 2 }));
    setPublicReason((r) => r || (same ? "Банк одобрил заявку на запрошенных условиях." : "Банк рассчитал условия, которые бизнес сможет обслуживать по своему денежному потоку."));
    setComment((cm) => cm || `Подтверждён расчётный вариант ${c.candidate_id.slice(0, 12)}.`);
  }

  // «Предложить этот вариант» in the assessment panel fills the form with the calculated terms.
  useEffect(() => {
    if (prefill) applyCandidate(prefill.candidate);
  }, [prefill?.nonce]); // eslint-disable-line react-hooks/exhaustive-deps

  // A recheck replaces the options: a picked option that is gone can no longer be offered.
  const candidates = basis?.candidates ?? [];
  const picked = candidates.find((c) => c.candidate_id === candidateId) ?? null;

  const withOffer = outcome === "APPROVE" || outcome === "COUNTER";
  const needsReason = outcome === "REJECT" || outcome === "COUNTER";
  const finalProduct = productId === "other" ? otherProduct.trim() : productId;
  const rateBps = parsePercent(ratePct);
  const termN = Number(term);

  const problems: string[] = [];
  if (!outcome) problems.push("Выберите решение.");
  if (withOffer && basis) {
    if (!candidates.length) problems.push("Предложить можно только рассчитанный вариант, а его сейчас нет: запросите данные или откажите.");
    else if (!picked) problems.push("Выберите расчётный вариант.");
  } else if (withOffer) {
    if (!/^[0-9a-f-]{36}$/i.test(finalProduct)) problems.push("Укажите продукт оферты.");
    if (amount === null || amount <= 0) problems.push("Укажите сумму оферты.");
    if (!Number.isInteger(termN) || termN < 1) problems.push("Укажите срок в месяцах.");
    if (rateBps === null) problems.push("Укажите ставку, например 19 или 18,5.");
  }
  if (outcome === "REQUEST_INFO") {
    if (!message.trim()) problems.push("Напишите клиенту, что нужно уточнить.");
    if (plan ? !requirementIds.length : !fields.length && !docs.length) problems.push("Отметьте хотя бы одно поле или документ.");
  }
  if (needsReason && !publicReason.trim()) problems.push("Для клиента нужна понятная причина решения.");
  if (!comment.trim()) problems.push("Добавьте внутренний комментарий.");

  async function submit() {
    setShown(true);
    if (problems.length || !outcome) return;
    const body: AnalystDecisionRequest = {
      outcome,
      comment: comment.trim(),
      // The reason field is hidden for REQUEST_INFO; text typed under another outcome must not leak.
      ...(outcome !== "REQUEST_INFO" && publicReason.trim() ? { public_reason: publicReason.trim() } : {}),
      ...(withOffer ? { offer: { product_id: finalProduct, amount: { amount_kopecks: amount!, currency: "RUB" }, term_months: termN, rate_bps: rateBps! } } : {}),
      // Profile v2 cases ask by requirement IDs from the server plan; legacy arrays stay empty.
      ...(outcome === "REQUEST_INFO"
        ? {
            info_request: plan
              ? { message: message.trim(), required_fields: [], required_document_types: [], requirement_ids: requirementIds }
              : { message: message.trim(), required_fields: fields, required_document_types: docs },
          }
        : {}),
    };
    const next = await act.run(async () => {
      try {
        return await decisionIntent.send((key) =>
          call(
            bankApi.POST("/backoffice/applications/{application_id}/decision", {
              params: { path: { application_id: kase.id }, header: { "Idempotency-Key": key, "If-Match": etag(kase.version) } },
              body,
            }),
          ),
        );
      } catch (e) {
        // A newer version of the case exists: refresh it so the analyst decides on current data.
        if (e instanceof ApiError && e.status === 409) void reload();
        throw e;
      }
    });
    if (next) onUpdated(next);
  }

  const toggle = <T,>(list: T[], value: T) => (list.includes(value) ? list.filter((v) => v !== value) : [...list, value]);

  return (
    <form
      className="panel side-panel decision"
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <fieldset className="group">
        <legend>
          <h2>Решение</h2>
        </legend>
        <Choices
          name="outcome"
          className="outcomes"
          options={outcomes}
          value={outcome}
          optionClassName={(value, on) => `outcome-opt outcome-${value}${on ? " on" : ""}`}
          onChange={setOutcome}
        />
      </fieldset>

      {withOffer && basis && (
        <fieldset className="sub-form">
          <legend>Расчётный вариант</legend>
          {candidates.length > 0 ? (
            <>
              <Choices
                name="candidate"
                className="candidate-picks"
                options={candidates.map((c) => ({
                  value: c.candidate_id,
                  label: money(rub(c.schedule.amount_kopecks)),
                  hint: `${months(c.schedule.term_months)}, ${rate(c.schedule.rate_bps)}, платёж ${money(rub(c.schedule.payment_kopecks))}. ${c.product_name}`,
                }))}
                value={picked?.candidate_id ?? null}
                optionClassName={(_, on) => `candidate-pick${on ? " on" : ""}`}
                onChange={(cid) => {
                  const c = candidates.find((x) => x.candidate_id === cid);
                  if (c) applyCandidate(c);
                }}
              />
              <p className="field-hint">Банк принимает оферту только с продуктом, суммой, сроком и ставкой из актуального расчёта.</p>
            </>
          ) : (
            <p className="note note-attention">
              {basis.stale
                ? "Оценка устарела: пересчитайте её, чтобы получить актуальные варианты."
                : "Расчёт не нашёл условий с посильным платежом. Запросите у клиента данные или откажите; после новых данных пересчитайте оценку."}
            </p>
          )}
        </fieldset>
      )}

      {withOffer && !basis && (
        <fieldset className="sub-form">
          <legend>Условия оферты</legend>
          <Field label="Продукт" htmlFor="d-product">
            <select id="d-product" value={productId} onChange={(e) => setProductId(e.target.value)}>
              {products ? (
                products.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                    {p.id === kase.product_id ? " (из заявки)" : ""}
                  </option>
                ))
              ) : (
                <>
                  <option value={kase.product_id}>{kase.product_name} (из заявки)</option>
                  {kase.offer && kase.offer.product_id !== kase.product_id && <option value={kase.offer.product_id}>{kase.offer.product_name}</option>}
                  <option value="other">Другой продукт…</option>
                </>
              )}
            </select>
          </Field>
          {productId === "other" && (
            <Field label="ID продукта" htmlFor="d-other" hint="UUID продукта из каталога банка">
              <input id="d-other" value={otherProduct} onChange={(e) => setOtherProduct(e.target.value)} />
            </Field>
          )}
          <Field label="Сумма" htmlFor="d-amount">
            <MoneyInput id="d-amount" value={amount ?? undefined} onChange={setAmount} />
          </Field>
          <div className="form-row">
            <Field label="Срок, мес." htmlFor="d-term">
              <input id="d-term" type="number" min={1} value={term} onChange={(e) => setTerm(e.target.value)} />
            </Field>
            <Field label="Ставка, %" htmlFor="d-rate">
              <input id="d-rate" inputMode="decimal" placeholder="19" value={ratePct} onChange={(e) => setRatePct(e.target.value)} />
            </Field>
          </div>
        </fieldset>
      )}

      {outcome === "REQUEST_INFO" && (
        <fieldset className="sub-form">
          <legend>Что запросить</legend>
          <Field label="Сообщение клиенту" htmlFor="d-msg">
            <textarea id="d-msg" rows={3} value={message} onChange={(e) => setMessage(e.target.value)} placeholder="Пришлите, пожалуйста, выписку за последние 12 месяцев" />
          </Field>
          {plan ? (
            <div className="checks">
              {plan.items.map((r) => (
                <label key={r.id} className="check">
                  <input type="checkbox" checked={requirementIds.includes(r.id)} onChange={() => setRequirementIds(toggle(requirementIds, r.id))} />
                  <span>
                    {r.label}
                    {r.period ? ` (${r.period.from} — ${r.period.to})` : ""}
                    {r.status === "PROVIDED" && <em className="tag tag-ok">есть</em>}
                  </span>
                </label>
              ))}
            </div>
          ) : (
          <div className="checks">
            {infoFields.map((f) => (
              <label key={f} className="check">
                <input type="checkbox" checked={fields.includes(f)} onChange={() => setFields(toggle(fields, f))} />
                {infoFieldLabel[f]}
              </label>
            ))}
            {documentTypes.map((d) => (
              <label key={d} className="check">
                <input type="checkbox" checked={docs.includes(d)} onChange={() => setDocs(toggle(docs, d))} />
                {documentLabel[d]}
              </label>
            ))}
          </div>
          )}
        </fieldset>
      )}

      {outcome && outcome !== "REQUEST_INFO" && (
        <Field label={needsReason ? "Причина для клиента" : "Пояснение для клиента, необязательно"} htmlFor="d-public" hint="Клиент увидит этот текст. Не упоминайте score и внутренние правила.">
          <textarea id="d-public" rows={3} maxLength={500} value={publicReason} onChange={(e) => setPublicReason(e.target.value)} />
        </Field>
      )}
      <Field label="Внутренний комментарий" htmlFor="d-comment" hint="Останется в истории дела, клиент его не видит.">
        <textarea id="d-comment" rows={3} maxLength={2000} value={comment} onChange={(e) => setComment(e.target.value)} />
      </Field>

      {shown && problems.length > 0 && (
        <ul className="note note-attention problems">
          {problems.map((p) => (
            <li key={p}>{p}</li>
          ))}
        </ul>
      )}
      <ErrorNote error={act.error} />
      <button className="btn btn-primary btn-block" disabled={act.busy}>
        {act.busy ? "Сохраняем…" : "Зафиксировать решение"}
      </button>
    </form>
  );
}
