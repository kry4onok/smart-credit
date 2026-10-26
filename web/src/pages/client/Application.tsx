import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { assessmentApi, clientApi } from "../../api/http";
import { ApiError, call } from "../../api/problem";
import type { AssessmentPreferences, ApplicationFormPatch, DocumentMetadata, Requirement, ClientApplication, CompanyProfile, InfoField, Offer, Product, PurposeTag, TimelineEntry } from "../../api/types";
import { useSession } from "../../auth/session";
import { DocumentSlot, MoneyInput } from "../../components/inputs";
import { CompareBars, offerRows } from "../../components/Compare";
import { ClientPipeline } from "../../components/Pipeline";
import { Badge, Choices, Dl, ErrorNote, Field, Loading, useAction, useNow, useTitle } from "../../components/ui";
import { date, dateTime, money, moneyShort, monthlyPayment, months, rate, rub, timeLeft } from "../../lib/format";
import { clientStatusLabel, clientStatusTone, documentLabel, infoFieldLabel, legalFormLabel, purposeLabel, purposeOrder, taxRegimeLabel } from "../../lib/labels";
import { PURPOSE_DESCRIPTION_MAX, latestDocuments, validateInfoValue, validatePurposeDescription } from "../../lib/applicationValidation";
import { usePolling, useIntent } from "../../lib/util";
import { forgetDraft, loadProducts, recallDraft, recallOffer, rememberDraft, rememberOffer } from "./products";
import { HintsPanel, PreferencesSection, RequirementsSection } from "./DraftExtras";
import { AssessmentView } from "./AssessmentView";
import { loanHints, monthsBetween } from "../../lib/hints";

const settled = new Set(["DRAFT", "DISBURSED", "REJECTED", "CLOSED"]);

export function ApplicationPage() {
  const { id = "" } = useParams();
  const [app, setApp] = useState<ClientApplication | null>(null);
  const [product, setProduct] = useState<Product | null>(null);
  const [productError, setProductError] = useState<unknown>(null);
  const [error, setError] = useState<unknown>(null);

  // Every response goes through apply: an older version never replaces a newer one
  // (a slow poll or PATCH answer must not roll back an upload or a later save).
  const apply = useCallback((next: ClientApplication) => {
    setApp((prev) => (prev && prev.id === next.id && prev.version > next.version ? prev : next));
  }, []);

  const fetchApp = useCallback(
    () => call(clientApi.GET("/client/applications/{application_id}", { params: { path: { application_id: id } } })),
    [id],
  );

  const load = useCallback(async () => {
    try {
      apply(await fetchApp());
      setError(null);
    } catch (e) {
      setError(e);
    }
  }, [apply, fetchApp]);

  useEffect(() => {
    setApp(null);
    void load();
  }, [load]);

  const productId = app?.product_id;
  const loadProduct = useCallback(() => {
    if (!productId) return;
    setProductError(null);
    loadProducts()
      .then((items) => {
        const found = items.find((p) => p.id === productId);
        setProduct(found ?? null);
        if (!found) setProductError(new ApiError(404, "Продукт этой заявки больше не в каталоге. Создайте заявку по другому продукту."));
      })
      .catch(setProductError);
  }, [productId]);
  useEffect(loadProduct, [loadProduct]);

  const waiting = !!app && (!settled.has(app.status) || !!app.pending_action);
  useTitle(app ? `Заявка: ${clientStatusLabel[app.status].toLowerCase()}` : "Заявка");
  usePolling(load, 3000, waiting);

  if (error && !app) return <ErrorNote error={error} onRetry={load} />;
  if (!app) return <Loading />;

  // Until all three fields are set the server has no form; the header shows what was typed.
  const local = app.status === "DRAFT" && !app.form ? recallDraft(app.id) : null;
  const amount = app.form?.amount ?? (local?.amount ? rub(local.amount) : undefined);
  // After acceptance the header shows the terms being issued; the request stays as context.
  const issued = app.status === "FORMALIZING" || app.status === "DISBURSED" ? (app.offer ?? recallOffer(app.id)) : null;
  return (
    <>
      <Link to="/cabinet" className="back">
        Все заявки
      </Link>
      <section className="app-head">
        <div>
          <p className="app-head-product">{issued?.product_name ?? product?.name ?? "Кредит для бизнеса"}</p>
          <h1 className="amount">{issued ? money(issued.amount) : amount?.amount_kopecks ? money(amount) : "Новая заявка"}</h1>
          <p className="app-head-meta">
            {issued
              ? `${months(issued.term_months)} под ${rate(issued.rate_bps)}${amount?.amount_kopecks ? `, запрошено ${money(amount)}` : ""}`
              : app.form?.term_months && app.form.purpose
                ? `${months(app.form.term_months)}, ${purposeLabel[app.form.purpose].toLowerCase()}`
                : "Заполните условия кредита"}
          </p>
        </div>
        <Badge tone={clientStatusTone[app.status]}>{clientStatusLabel[app.status]}</Badge>
      </section>
      <ClientPipeline status={app.status} waiting={waiting && app.status !== "DRAFT"} />

      <section className="next panel">
        {/* Polling changes the status without a click; the live region announces it. */}
        <div aria-live="polite">
          <h2>{app.stage_label}</h2>
          {app.next_action && <p className="next-action">{app.next_action}</p>}
        </div>
        <ErrorNote error={error} />
        {app.status === "DRAFT" && <ErrorNote error={productError} onRetry={loadProduct} />}
        <NextStep app={app} product={product} onChange={apply} reload={load} fetchApp={fetchApp} />
      </section>

      {app.status !== "DRAFT" && <AssessmentView app={app} />}
      {app.status !== "DRAFT" && <Summary app={app} />}
      {app.status !== "DRAFT" && <Timeline id={app.id} version={app.version} />}
    </>
  );
}

type StepProps = {
  app: ClientApplication;
  product: Product | null;
  onChange: (a: ClientApplication) => void;
  reload: () => Promise<void>;
  fetchApp: () => Promise<ClientApplication>;
};

function NextStep({ app, product, onChange, reload, fetchApp }: StepProps) {
  if (app.status === "DRAFT") return <DraftEditor app={app} product={product} onChange={onChange} fetchApp={fetchApp} />;
  if (app.pending_action) return <Pending action={app.pending_action} />;
  return (
    <>
      {app.status === "NEEDS_INFO" && app.info_request && app.allowed_actions.includes("PROVIDE_INFO") && (
        <InfoResponse key={app.info_request.id} app={app} onChange={onChange} reload={reload} />
      )}
      {(app.allowed_actions.includes("ACCEPT_OFFER") || app.allowed_actions.includes("SIGN")) && <OfferStep app={app} reload={reload} />}
      {app.status === "DISBURSED" && (
        <div className="outcome outcome-success">
          <strong>Кредит выдан.</strong> Это симуляция: деньги не переводились, но все этапы конвейера пройдены.
        </div>
      )}
      {(app.status === "REJECTED" || app.status === "CLOSED") && app.public_reason && (
        <div className={`outcome ${app.status === "REJECTED" ? "outcome-danger" : ""}`}>
          <p>{app.public_reason}</p>
        </div>
      )}
      {app.allowed_actions.includes("CANCEL") && <CancelButton app={app} reload={reload} />}
    </>
  );
}

const pendingText: Record<string, string> = {
  INFO_SENT: "Ответ отправлен. Банк проверяет новые данные — статус обновится сам.",
  ACCEPT_PENDING: "Согласие отправлено. Банк готовит документы для подписания.",
  DECLINE_PENDING: "Отказ от предложения отправлен. Банк закроет заявку.",
  SIGN_PENDING: "Подпись отправлена. Банк проводит выдачу.",
  CANCEL_PENDING: "Запрос на отзыв отправлен. Банк закроет заявку.",
};

function Pending({ action }: { action: string }) {
  return (
    <p className="pending" role="status">
      <span className="spinner" aria-hidden />
      {pendingText[action] ?? "Действие отправлено банку."}
    </p>
  );
}

type FormState = { amount: number | null | undefined; term: number | undefined; purpose: PurposeTag | undefined; description: string | undefined };

function DraftEditor({ app, product, onChange, fetchApp }: Omit<StepProps, "reload">) {
  const { user } = useSession();
  const [form, setForm] = useState<FormState>(() => {
    const saved = recallDraft(app.id);
    return {
      amount: app.form?.amount?.amount_kopecks ?? saved.amount ?? undefined,
      term: app.form?.term_months ?? saved.term,
      purpose: app.form?.purpose ?? saved.purpose ?? (product?.purpose_tags.length === 1 ? product.purpose_tags[0] : undefined),
      // An incomplete draft has no form yet; the explanation then comes from the top level.
      description: app.form?.purpose_description ?? app.purpose_description ?? saved.description,
    };
  });
  const [saveState, setSaveState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [savedAt, setSavedAt] = useState<Date | null>(null);
  const navigate = useNavigate();
  const [saveError, setSaveError] = useState<unknown>(null);
  const submit = useAction();
  const submitIntent = useIntent();
  const latest = useRef(app);
  if (app.version >= latest.current.version) latest.current = app;
  const queue = useRef<ApplicationFormPatch | null>(null);
  // Assessment preferences (priority, comfortable payment…) share the queue: both bump the card version.
  const [prefs, setPrefs] = useState<AssessmentPreferences | null>(null);
  const prefsQueue = useRef<AssessmentPreferences | null>(null);
  const inflight = useRef<Promise<boolean> | null>(null);
  const failed = useRef(false);
  const timer = useRef<number | undefined>(undefined);

  const refresh = useCallback(async () => {
    const fresh = await fetchApp();
    if (fresh.version >= latest.current.version) latest.current = fresh;
    onChange(fresh);
  }, [fetchApp, onChange]);

  const patchOnce = useCallback(
    (patch: ApplicationFormPatch) =>
      call(
        clientApi.PATCH("/client/applications/{application_id}", {
          params: { path: { application_id: latest.current.id }, header: { "If-Match": `"${latest.current.version}"` } },
          body: patch,
          headers: { "Content-Type": "application/merge-patch+json" },
        }),
      ),
    [],
  );

  // Sends one PATCH. A 409 means the draft changed elsewhere (for example a document upload
  // raised its version): take the fresh version and repeat once. On failure the patch goes back
  // to the queue under any newer edits, so nothing typed is lost and «Повторить» resends it.
  const save = useCallback(
    async (patch: ApplicationFormPatch): Promise<boolean> => {
      setSaveState("saving");
      try {
        let next: ClientApplication;
        try {
          next = await patchOnce(patch);
        } catch (e) {
          if (!(e instanceof ApiError && e.status === 409)) throw e;
          await refresh();
          next = await patchOnce(patch);
        }
        latest.current = next;
        onChange(next);
        setSaveState("saved");
        setSavedAt(new Date());
        setSaveError(null);
        return true;
      } catch (e) {
        queue.current = { ...patch, ...queue.current };
        failed.current = true;
        setSaveState("error");
        setSaveError(e);
        return false;
      }
    },
    [onChange, patchOnce, refresh],
  );

  const putPrefs = useCallback(
    (preferences: AssessmentPreferences) =>
      call(
        assessmentApi.PUT("/client/applications/{application_id}/assessment-input", {
          params: { path: { application_id: latest.current.id }, header: { "If-Match": `"${latest.current.version}"` } },
          body: { schema_version: 1, preferences },
        }),
      ),
    [],
  );

  const savePrefs = useCallback(
    async (preferences: AssessmentPreferences): Promise<boolean> => {
      setSaveState("saving");
      try {
        let saved;
        try {
          saved = await putPrefs(preferences);
        } catch (e) {
          if (!(e instanceof ApiError && e.status === 409)) throw e;
          await refresh();
          saved = await putPrefs(preferences);
        }
        latest.current = { ...latest.current, version: saved.version };
        onChange(latest.current);
        setSaveState("saved");
        setSavedAt(new Date());
        setSaveError(null);
        return true;
      } catch (e) {
        prefsQueue.current ??= preferences;
        failed.current = true;
        setSaveState("error");
        setSaveError(e);
        return false;
      }
    },
    [onChange, putPrefs, refresh],
  );

  // Drains the queue one request at a time; every caller waits until nothing is left in flight.
  const flush = useCallback(async (): Promise<boolean> => {
    window.clearTimeout(timer.current);
    while (!failed.current && (inflight.current || queue.current || prefsQueue.current)) {
      if (inflight.current) {
        await inflight.current;
        continue;
      }
      if (queue.current) {
        const patch = queue.current;
        queue.current = null;
        inflight.current = save(patch);
      } else {
        const preferences = prefsQueue.current!;
        prefsQueue.current = null;
        inflight.current = savePrefs(preferences);
      }
      inflight.current = inflight.current.finally(() => {
        inflight.current = null;
      });
    }
    return !failed.current;
  }, [save, savePrefs]);

  // Products with the cash-flow assessment accept preferences; for others the endpoint is absent.
  useEffect(() => {
    call(assessmentApi.GET("/client/applications/{application_id}/assessment-input", { params: { path: { application_id: app.id } } }))
      .then((env) => setPrefs(env.assessment_input.preferences ?? {}))
      .catch(() => setPrefs(null));
  }, [app.id]);

  function updatePrefs(next: AssessmentPreferences) {
    setPrefs(next);
    prefsQueue.current = next;
    failed.current = false;
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => void flush(), 900);
  }

  const retry = () => {
    failed.current = false;
    void flush();
  };

  // Leaving the page within the autosave delay must not drop the last edit.
  useEffect(() => () => void flush(), [flush]);

  // A product with a single purpose needs no choice; the product may load after the editor mounts.
  useEffect(() => {
    if (form.purpose === undefined && product?.purpose_tags.length === 1) update({ purpose: product.purpose_tags[0] }, { purpose: product.purpose_tags[0] });
  }, [product]); // eslint-disable-line react-hooks/exhaustive-deps

  function update(next: Partial<FormState>, patch: ApplicationFormPatch | null) {
    setForm((f) => {
      const merged = { ...f, ...next };
      rememberDraft(app.id, merged);
      return merged;
    });
    if (!patch) return;
    queue.current = { ...queue.current, ...patch };
    failed.current = false;
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => void flush(), 700);
  }

  // The product's purpose_tags are restrictions: only they can be chosen, and a purpose saved
  // earlier that the product no longer finances blocks submission until it is changed.
  const purposes = product ? purposeOrder.filter((p) => product.purpose_tags.includes(p)) : purposeOrder;
  const purposeError =
    form.purpose && product && !product.purpose_tags.includes(form.purpose)
      ? `Продукт «${product.name}» не финансирует цель «${purposeLabel[form.purpose]}». Выберите одну из доступных целей.`
      : null;
  const descriptionError = validatePurposeDescription(form.purpose, form.description);
  const amountError =
    form.amount === null
      ? "Введите сумму числом, например 1 500 000."
      : product && form.amount !== undefined && (form.amount < product.min_amount.amount_kopecks || form.amount > product.max_amount.amount_kopecks)
        ? `Для этого продукта сумма от ${moneyShort(product.min_amount)} до ${moneyShort(product.max_amount)}.`
        : null;
  const termError =
    form.term !== undefined && (!Number.isSafeInteger(form.term) || form.term <= 0 || form.term > 2147483647)
      ? "Укажите целый положительный срок в месяцах."
      : product && form.term !== undefined && (form.term < product.min_term_months || form.term > product.max_term_months)
      ? `Срок от ${product.min_term_months} до ${months(product.max_term_months)}.`
      : null;
  const required = product?.required_document_types ?? [];
  const missingDocs = required.filter((t) => !app.documents.some((d) => d.document_type === t));
  const missingFields = [
    form.amount == null && "сумма",
    form.term === undefined && "срок",
    form.purpose === undefined && "цель кредита",
    form.purpose === "OTHER" && !form.description?.trim() && "пояснение, на что пойдут деньги",
  ].filter(Boolean);
  const formReady = form.amount != null && form.term !== undefined && form.purpose !== undefined && !amountError && !termError && !purposeError && !descriptionError;
  const canSubmit =
    !!product && app.allowed_actions.includes("SUBMIT") && formReady && missingDocs.length === 0 && user?.can_submit !== false && saveState !== "error";

  // Deleting waits for autosave so If-Match carries the version the server has. Conflicts reload
  // the card and are reported; the delete is never retried automatically.
  async function remove(): Promise<string | null> {
    if (!(await flush())) await refresh().catch(() => undefined);
    try {
      await call(
        clientApi.DELETE("/client/applications/{application_id}", {
          params: { path: { application_id: latest.current.id }, header: { "If-Match": `"${latest.current.version}"` } },
        }),
      );
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        forgetDraft(app.id);
        navigate("/cabinet", { replace: true });
        return null;
      }
      if (e instanceof ApiError && e.status === 409) {
        await refresh().catch(() => undefined);
        return e.code === "APPLICATION_STATE_CONFLICT"
          ? "Заявка уже не черновик, удалить её нельзя. Карточка обновлена."
          : "Черновик успел измениться. Карточка обновлена — проверьте её и удалите ещё раз, если нужно.";
      }
      throw e;
    }
    window.clearTimeout(timer.current);
    queue.current = null;
    prefsQueue.current = null;
    forgetDraft(app.id);
    navigate("/cabinet", { replace: true });
    return null;
  }

  async function send() {
    const result = await submit.run(async () => {
      // The bank must receive exactly what is on screen: save first, submit only if that worked.
      if (!(await flush())) throw new ApiError(0, "Черновик не сохранился. Нажмите «Повторить» рядом с полями и отправьте снова.");
      return submitIntent.send((key) =>
        call(clientApi.POST("/client/applications/{application_id}/submit", { params: { path: { application_id: latest.current.id }, header: { "Idempotency-Key": key } } })),
      );
    });
    if (result) onChange(result);
  }

  const plan = app.requirements_plan;
  const company = app.company;
  const periodMonths = company.financial_period ? Math.max(1, monthsBetween(company.financial_period.from, new Date(company.financial_period.to)) + 1) : 12;
  const hints = loanHints({
    amountKopecks: form.amount ?? undefined,
    termMonths: form.term,
    rateBps: product?.indicative_rate_bps,
    profitKopecks: company.profit_12m?.amount_kopecks,
    revenueKopecks: company.revenue_12m?.amount_kopecks,
    obligationsKopecks: company.obligations?.amount_kopecks,
    periodMonths,
    registeredAt: company.registered_at,
    maxPaymentKopecks: prefs?.max_payment_kopecks,
    maxTerm: product?.max_term_months,
    minAmountKopecks: product?.min_amount.amount_kopecks,
  });
  const planReady = !plan || plan.can_submit;
  const missingSubmitDocs = plan ? plan.items.filter((r) => r.kind === "DOCUMENT" && r.required_at === "SUBMIT" && r.status === "MISSING") : [];
  const docsReady = plan ? missingSubmitDocs.length === 0 : missingDocs.length === 0;
  const ready = canSubmit && planReady && docsReady;
  let step = 0;
  const no = () => ++step;
  const onUploaded = (doc: DocumentMetadata) => {
    onChange({ ...latest.current, documents: [...latest.current.documents.filter((d) => d.id !== doc.id && (plan || d.document_type !== doc.document_type)), doc] });
    // The upload raised the draft version on the server; the next PATCH needs it in If-Match.
    void refresh().catch(() => undefined);
  };

  return (
    <div className="draft">
      <DraftToolbar
        canEdit={app.allowed_actions.includes("EDIT_DRAFT")}
        canDelete={app.allowed_actions.includes("DELETE_DRAFT")}
        saveState={saveState}
        savedAt={savedAt}
        saveError={saveError}
        onRetry={retry}
        onDelete={remove}
      />
      <fieldset className="draft-step" id="draft-terms">
        <legend>
          <span className="step-no">{no()}</span> Условия кредита
        </legend>
        <div className="form-grid">
          <Field
            label="Сумма"
            htmlFor="amount"
            error={amountError}
            hint={product ? `от ${moneyShort(product.min_amount)} до ${moneyShort(product.max_amount)}` : undefined}
          >
            <MoneyInput id="amount" value={form.amount ?? undefined} invalid={!!amountError} onChange={(v) => update({ amount: v ?? null }, v == null ? null : { amount: rub(v) })} />
          </Field>
          <Field label="Срок, месяцев" htmlFor="term" error={termError} hint={product ? `от ${product.min_term_months} до ${product.max_term_months}` : undefined}>
            <input
              id="term"
              type="number"
              inputMode="numeric"
              min={product?.min_term_months ?? 1}
              max={product?.max_term_months}
              value={form.term ?? ""}
              aria-invalid={!!termError}
              onChange={(e) => {
                const n = e.target.value === "" ? undefined : Number(e.target.value);
                update({ term: n }, n !== undefined && Number.isSafeInteger(n) && n > 0 && n <= 2147483647 ? { term_months: n } : null);
              }}
            />
          </Field>
          <Field label="Цель" group error={purposeError}>
            <Choices
              name="purpose"
              className="segmented"
              options={purposes.map((p) => ({ value: p, label: <span>{purposeLabel[p]}</span> }))}
              value={form.purpose}
              optionClassName={(_, on) => (on ? "on" : "")}
              onChange={(p) => update({ purpose: p }, { purpose: p })}
            />
          </Field>
        </div>
        <Field
          label="На что планируете потратить деньги"
          htmlFor="purpose-description"
          error={form.purpose === "OTHER" || (form.description?.length ?? 0) > 0 ? descriptionError : null}
          hint={`${form.purpose === "OTHER" ? "Обязательно для цели «Несколько целей или другая цель»" : "Необязательно"}. ${[...(form.description ?? "")].length} из ${PURPOSE_DESCRIPTION_MAX} символов.`}
        >
          <textarea
            id="purpose-description"
            rows={3}
            maxLength={PURPOSE_DESCRIPTION_MAX}
            value={form.description ?? ""}
            aria-invalid={form.purpose === "OTHER" && !!descriptionError}
            placeholder="Например: ремонт помещения, покупка оборудования и закупка сырья"
            onChange={(e) => update({ description: e.target.value }, { purpose_description: e.target.value })}
          />
        </Field>
        {formReady && product?.indicative_rate_bps !== undefined && (
          <p className="estimate">
            Ориентировочный платёж — {money(monthlyPayment(rub(form.amount!), form.term!, product.indicative_rate_bps))} в месяц при ставке{" "}
            {rate(product.indicative_rate_bps)}. Это предварительный расчёт, не оферта.
          </p>
        )}
        <HintsPanel
          hints={hints}
          onFix={(fix) => {
            if (fix.amountKopecks) update({ amount: fix.amountKopecks }, { amount: rub(fix.amountKopecks) });
            if (fix.termMonths) update({ term: fix.termMonths }, { term_months: fix.termMonths });
          }}
        />
      </fieldset>

      {prefs && (
        <fieldset className="draft-step">
          <legend>
            <span className="step-no">{no()}</span> Если всю сумму дать не получится
          </legend>
          <PreferencesSection prefs={prefs} onChange={updatePrefs} amountKopecks={form.amount ?? undefined} />
        </fieldset>
      )}

      <fieldset className="draft-step">
        <legend>
          <span className="step-no">{no()}</span> Бизнес
        </legend>
        <CompanySnapshot company={app.company} />
        <p className="field-hint">
          Сведения взяты из <Link to="/cabinet/company">профиля бизнеса</Link> в момент создания черновика.
        </p>
      </fieldset>

      <fieldset className="draft-step">
        <legend>
          <span className="step-no">{no()}</span> Документы
        </legend>
        {plan ? (
          <RequirementsSection
            applicationId={app.id}
            plan={plan}
            legalForm={app.company.legal_form}
            documents={app.documents}
            canUpload={app.allowed_actions.includes("UPLOAD_DOCUMENT")}
            onUploaded={onUploaded}
          />
        ) : (
          <>
            {required.length === 0 && <p className="field-hint">Для этого продукта документы не нужны.</p>}
            {required.map((type) => (
              <DocumentSlot
                key={type}
                applicationId={app.id}
                type={type}
                legalForm={app.company.legal_form}
                documents={app.documents}
                enabled={app.allowed_actions.includes("UPLOAD_DOCUMENT")}
                onUploaded={onUploaded}
              />
            ))}
          </>
        )}
        <p className="field-hint">Файл сохраняется как демо-документ. Для проверки достаточно любого текстового файла .txt. Суммы и реквизиты из файлов не извлекаются автоматически — их нужно указать в форме. Банк проверяет наличие документа.</p>
      </fieldset>

      <fieldset className="draft-step">
        <legend>
          <span className="step-no">{no()}</span> Отправка в банк
        </legend>
        <ul className="checklist">
          <li className={formReady ? "ok" : ""}>{form.purpose === "OTHER" ? "Сумма, срок, цель и её пояснение заполнены" : "Сумма, срок и цель заполнены"}</li>
          <li className={docsReady ? "ok" : ""}>{plan ? "Документы для отправки приложены" : "Документы приложены"}</li>
          {plan && <li className={plan.can_submit ? "ok" : ""}>{plan.can_submit ? "Банк готов принять заявку" : "Заполните недостающие сведения"}</li>}
          <li className={user?.can_submit !== false ? "ok" : ""}>
            {user?.can_submit !== false ? "У вас есть право отправлять заявки" : "Отправить заявку может только уполномоченный сотрудник"}
          </li>
        </ul>
        {missingFields.length > 0 && <p className="field-hint" role="status">До отправки заполните: {missingFields.join(", ")}. Заявка останется черновиком, пока вы её не отправите.</p>}
        {!plan && missingDocs.length > 0 && <p className="field-hint" role="status">Приложите: {missingDocs.map((type) => documentLabel[type]).join(", ")}.</p>}
        {hints.some((h) => h.level === "bad") && <p className="field-hint">Заявку можно отправить и так — подсказки выше показывают, на что банк обратит внимание.</p>}
        <ErrorNote error={submit.error} />
        <button type="button" className="btn btn-primary" disabled={!ready || submit.busy} onClick={() => void send()}>
          {submit.busy ? "Отправляем…" : "Отправить заявку"}
        </button>
      </fieldset>
    </div>
  );
}

type ToolbarProps = {
  canEdit: boolean;
  canDelete: boolean;
  saveState: "idle" | "saving" | "saved" | "error";
  savedAt: Date | null;
  saveError: unknown;
  onRetry: () => void;
  // Resolves to a message when the draft was not deleted (it changed meanwhile).
  onDelete: () => Promise<string | null>;
};

// DraftToolbar keeps draft-level actions in one place: where to edit, whether edits are saved,
// and deletion behind an explicit confirmation.
function DraftToolbar({ canEdit, canDelete, saveState, savedAt, saveError, onRetry, onDelete }: ToolbarProps) {
  const [confirming, setConfirming] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const del = useAction();
  const status =
    saveState === "saving"
      ? "Сохраняем…"
      : saveState === "error"
        ? "Изменения не сохранились"
        : savedAt
          ? `Сохранено в ${savedAt.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" })}`
          : "Изменения сохраняются автоматически";

  async function confirmDelete() {
    const message = await del.run(onDelete);
    if (message) {
      setNotice(message);
      setConfirming(false);
    }
  }

  return (
    <div className="draft-toolbar panel">
      <div className="draft-toolbar-row">
        <p className={`autosave autosave-${saveState}`} role="status" aria-live="polite">
          {status}
        </p>
        <div className="draft-toolbar-actions">
          {canEdit && (
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={() => {
                document.getElementById("draft-terms")?.scrollIntoView({ behavior: "smooth", block: "start" });
                document.getElementById("amount")?.focus({ preventScroll: true });
              }}
            >
              Редактировать черновик
            </button>
          )}
          {canDelete && !confirming && (
            <button type="button" className="btn btn-quiet btn-sm" onClick={() => setConfirming(true)}>
              Удалить черновик
            </button>
          )}
        </div>
      </div>
      {saveState === "error" && <ErrorNote error={saveError} onRetry={onRetry} />}
      {confirming && (
        <div className="note note-danger" role="alertdialog" aria-labelledby="delete-draft-title">
          <p id="delete-draft-title">
            <strong>Удалить черновик?</strong> Заявка и все приложенные к ней файлы будут удалены. Восстановить их не получится.
          </p>
          <div className="draft-toolbar-actions">
            <button type="button" className="btn btn-danger btn-sm" disabled={del.busy} onClick={() => void confirmDelete()}>
              {del.busy ? "Удаляем…" : "Удалить"}
            </button>
            <button type="button" className="btn btn-quiet btn-sm" disabled={del.busy} onClick={() => setConfirming(false)}>
              Отмена
            </button>
          </div>
        </div>
      )}
      <ErrorNote error={del.error} />
      {notice && (
        <p className="note note-attention" role="status">
          {notice}
        </p>
      )}
    </div>
  );
}

function CompanySnapshot({ company }: { company: CompanyProfile }) {
  return (
    <Dl
      items={[
        ["Компания", company.name],
        ...(company.legal_form ? ([["Форма", legalFormLabel[company.legal_form]]] as [string, string][]) : []),
        ["ИНН", company.inn],
        ...(company.tax_periods?.length
          ? ([["Налоговый режим", company.tax_periods[company.tax_periods.length - 1].regimes.map((r) => taxRegimeLabel[r] ?? r).join(", ")]] as [string, string][])
          : []),
        ["ОКВЭД", company.okved],
        ["Зарегистрирована", date(company.registered_at)],
        ["Выручка за 12 мес.", money(company.revenue_12m)],
        ["Прибыль за 12 мес.", money(company.profit_12m)],
        ["Кредиты и займы", money(company.obligations)],
      ]}
    />
  );
}

// Requirement field paths that the legacy inputs already know how to edit.
const fieldByPath: Record<string, InfoField> = {
  "company.revenue_12m": "COMPANY_REVENUE_12M",
  "company.profit_12m": "COMPANY_PROFIT_12M",
  "company.obligations": "COMPANY_OBLIGATIONS",
  "form.amount": "APPLICATION_AMOUNT",
  "form.term_months": "APPLICATION_TERM_MONTHS",
  "form.purpose": "APPLICATION_PURPOSE",
};

const companyFields: Partial<Record<InfoField, "revenue_12m" | "profit_12m" | "obligations">> = {
  COMPANY_REVENUE_12M: "revenue_12m",
  COMPANY_PROFIT_12M: "profit_12m",
  COMPANY_OBLIGATIONS: "obligations",
};

function InfoResponse({ app, onChange, reload }: { app: ClientApplication; onChange: (a: ClientApplication) => void; reload: () => Promise<void> }) {
  const request = app.info_request!;
  const [values, setValues] = useState<Partial<Record<InfoField, number | null | string>>>(() => ({
    COMPANY_REVENUE_12M: app.company.revenue_12m?.amount_kopecks,
    COMPANY_PROFIT_12M: app.company.profit_12m?.amount_kopecks,
    COMPANY_OBLIGATIONS: app.company.obligations?.amount_kopecks,
    APPLICATION_AMOUNT: app.form?.amount?.amount_kopecks,
    APPLICATION_TERM_MONTHS: app.form?.term_months,
    APPLICATION_PURPOSE: app.form?.purpose,
  }));
  const [description, setDescription] = useState(app.form?.purpose_description ?? app.purpose_description ?? "");
  const documents = latestDocuments(app.documents);
  const send = useAction();
  const intent = useIntent();
  // Profile v2 requests name requirements (documents with periods, fields by path); legacy ones use the two arrays.
  const requirements = request.requirements ?? [];
  const v2 = requirements.length > 0;
  const docRequirements = requirements.filter((r) => r.kind === "DOCUMENT");
  const requestedFields: InfoField[] = v2
    ? requirements.filter((r) => r.kind === "FIELD" && r.field_path && fieldByPath[r.field_path]).map((r) => fieldByPath[r.field_path!])
    : request.required_fields;
  const otherFields = v2 ? requirements.filter((r) => r.kind === "FIELD" && (!r.field_path || !fieldByPath[r.field_path])) : [];
  const attachedFor = (r: Requirement) => app.documents.filter((d) => d.requirement_id === r.id || r.document_ids.includes(d.id));

  const fieldErrors = Object.fromEntries(requestedFields.map((field) => [field, validateInfoValue(field, values[field])])) as Partial<Record<InfoField, string | null>>;
  const asksPurpose = requestedFields.includes("APPLICATION_PURPOSE");
  const descriptionError = asksPurpose ? validatePurposeDescription(values.APPLICATION_PURPOSE as string | undefined, description) : null;
  const invalidFields = requestedFields.filter((field) => fieldErrors[field] || (field === "APPLICATION_PURPOSE" && descriptionError));
  const missingDocuments = v2
    ? docRequirements.filter((r) => attachedFor(r).length === 0).map((r) => r.label)
    : request.required_document_types.filter((type) => !documents.some((document) => document.document_type === type)).map((t) => documentLabel[t]);

  async function submit() {
    if (invalidFields.length || missingDocuments.length) return;
    const company = { ...app.company };
    const fields: ApplicationFormPatch = {};
    for (const f of requestedFields) {
      const key = companyFields[f];
      if (key) company[key] = rub(values[f] as number);
      if (f === "APPLICATION_AMOUNT") fields.amount = rub(values[f] as number);
      if (f === "APPLICATION_TERM_MONTHS") fields.term_months = Number(values[f]);
      if (f === "APPLICATION_PURPOSE") {
        fields.purpose = values[f] as PurposeTag;
        // The explanation travels with the purpose; an empty string clears a previous one.
        fields.purpose_description = description;
      }
    }
    // The button stays busy until the refreshed card arrives, so a second click cannot resend.
    await send.run(async () => {
      await intent.send((key) =>
        call(
          clientApi.POST("/client/applications/{application_id}/info", {
            params: { path: { application_id: app.id }, header: { "Idempotency-Key": key } },
            body: {
              request_id: request.id,
              ...(requestedFields.some((f) => companyFields[f]) ? { company } : {}),
              ...(Object.keys(fields).length ? { fields } : {}),
              // Legacy: omit document_ids, the backend includes all attached document types.
              // Profile v2: name the files that answer each requirement.
              ...(v2 ? { document_ids: docRequirements.flatMap((r) => attachedFor(r).map((d) => d.id)) } : {}),
            },
          }),
        ),
      );
      await reload();
    });
  }

  return (
    <div className="info-request">
      <blockquote>{request.message}</blockquote>
      <p className="field-hint">Запрос от {dateTime(request.created_at)}. Изменения касаются только этой заявки.</p>
      <div className="form-grid">
        {requestedFields.map((f) => (
          <Field key={f} label={infoFieldLabel[f]} htmlFor={`info-${f}`} error={fieldErrors[f]}>
            {f === "APPLICATION_PURPOSE" ? (
              <select id={`info-${f}`} aria-invalid={!!fieldErrors[f]} value={(values[f] as string) ?? ""} onChange={(e) => setValues({ ...values, [f]: e.target.value })}>
                <option value="">Выберите цель кредита</option>
                {purposeOrder.map((p) => (
                  <option key={p} value={p}>
                    {purposeLabel[p]}
                  </option>
                ))}
              </select>
            ) : f === "APPLICATION_TERM_MONTHS" ? (
              <input id={`info-${f}`} type="number" min={1} step={1} max={2147483647} aria-invalid={!!fieldErrors[f]} value={(values[f] as number) ?? ""} onChange={(e) => setValues({ ...values, [f]: e.target.value === "" ? null : Number(e.target.value) })} />
            ) : (
              <MoneyInput id={`info-${f}`} invalid={!!fieldErrors[f]} allowNegative={f === "COMPANY_PROFIT_12M"} value={(values[f] as number) ?? undefined} onChange={(v) => setValues({ ...values, [f]: v ?? null })} />
            )}
          </Field>
        ))}
      </div>
      {asksPurpose && (
        <Field
          label="На что планируете потратить деньги"
          htmlFor="info-purpose-description"
          error={values.APPLICATION_PURPOSE === "OTHER" || description.length > 0 ? descriptionError : null}
          hint={`${values.APPLICATION_PURPOSE === "OTHER" ? "Обязательно для цели «Несколько целей или другая цель»" : "Необязательно"}. ${[...description].length} из ${PURPOSE_DESCRIPTION_MAX} символов.`}
        >
          <textarea id="info-purpose-description" rows={3} maxLength={PURPOSE_DESCRIPTION_MAX} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>
      )}
      {otherFields.length > 0 && (
        <p className="note note-attention">
          Банк просит уточнить: {otherFields.map((r) => r.label.toLowerCase()).join(", ")}. Обновите их в <Link to="/cabinet/company">сведениях о бизнесе</Link>.
        </p>
      )}
      {v2
        ? docRequirements.map((r) => (
            <DocumentSlot
              key={r.id}
              applicationId={app.id}
              requirement={r}
              legalForm={app.company.legal_form}
              documents={app.documents}
              enabled
              onUploaded={(doc) => {
                onChange({ ...app, documents: [...app.documents.filter((existing) => existing.id !== doc.id), doc] });
                void reload();
              }}
            />
          ))
        : request.required_document_types.map((type) => (
            <DocumentSlot
              key={type}
              applicationId={app.id}
              type={type}
              legalForm={app.company.legal_form}
              documents={documents}
              enabled
              onUploaded={(doc) => {
                onChange({ ...app, documents: [...app.documents.filter((existing) => existing.id !== doc.id), doc] });
                void reload();
              }}
            />
          ))}
      <p className="field-hint">Ответ отправляется только после заполнения всех запрошенных полей и загрузки документов. До этого заявка остаётся в статусе «Нужны данные»; файл сам не заполняет финансовые показатели.</p>
      {invalidFields.length > 0 && <p className="field-hint" role="status">Проверьте поля: {invalidFields.map((field) => infoFieldLabel[field]).join(", ")}.</p>}
      {missingDocuments.length > 0 && <p className="field-hint" role="status">Не хватает документов: {missingDocuments.join(", ")}.</p>}
      <ErrorNote error={send.error} />
      <button type="button" className="btn btn-primary" disabled={invalidFields.length > 0 || missingDocuments.length > 0 || send.busy} onClick={() => void submit()}>
        {send.busy ? "Отправляем…" : "Отправить ответ банку"}
      </button>
    </div>
  );
}

function OfferStep({ app, reload }: { app: ClientApplication; reload: () => Promise<void> }) {
  const offer = app.offer ?? recallOffer(app.id);
  if (offer) return <OfferView app={app} offer={offer} reload={reload} />;
  return (
    <p className="note note-attention">
      Предложение принято, но условия для подписи сейчас не загрузились. Обновите страницу чуть позже или обратитесь в банк.
    </p>
  );
}

function OfferView({ app, offer, reload }: { app: ClientApplication; offer: Offer; reload: () => Promise<void> }) {
  useNow();
  const act = useAction();
  const intents = { accept: useIntent(), decline: useIntent(), sign: useIntent() };
  const [confirmDecline, setConfirmDecline] = useState(false);
  const [simulated, setSimulated] = useState(false);
  const canAccept = app.allowed_actions.includes("ACCEPT_OFFER");
  const canSign = app.allowed_actions.includes("SIGN");
  const accepted = !canAccept && canSign;
  const counter = offer.kind === "COUNTER_OFFER";
  const differentProduct = offer.product_id !== app.product_id;

  const paths = {
    accept: "/client/applications/{application_id}/offer/accept",
    decline: "/client/applications/{application_id}/offer/decline",
    sign: "/client/applications/{application_id}/sign",
  } as const;

  async function post(action: keyof typeof paths) {
    await act.run(async () => {
      await intents[action].send((key) =>
        call(clientApi.POST(paths[action], { params: { path: { application_id: app.id }, header: { "Idempotency-Key": key } }, body: { offer_id: offer.id } })),
      );
      if (action === "accept") rememberOffer(app.id, offer);
      await reload();
    });
  }

  return (
    <div className={`offer${counter ? " offer-counter" : ""}`}>
      {accepted && <p className="offer-accepted">Вы приняли эти условия. Осталось подписать договор.</p>}
      {counter && !accepted && (
        <p className="offer-note">
          Банк не может выдать кредит на запрошенных условиях и предлагает {differentProduct ? "другой продукт" : "другие условия"}.
        </p>
      )}
      <div className="offer-grid">
        <div className="offer-main">
          <span>{offer.product_name}</span>
          <strong className="amount">{money(offer.amount)}</strong>
          <span>
            на {months(offer.term_months)} под {rate(offer.rate_bps)} годовых
          </span>
        </div>
        <Dl
          items={[
            ["Платёж в месяц", `≈ ${money(monthlyPayment(offer.amount, offer.term_months, offer.rate_bps))}`],
            ...(accepted ? [] : ([["Предложение действует", `до ${dateTime(offer.expires_at)}, ${timeLeft(offer.expires_at)}`]] as [string, string][])),
          ]}
        />
      </div>
      {app.form?.amount && app.form.term_months && (counter || app.form.amount.amount_kopecks !== offer.amount.amount_kopecks || app.form.term_months !== offer.term_months) && (
        <CompareBars rows={offerRows({ amount: app.form.amount, term_months: app.form.term_months }, offer)} />
      )}
      {offer.explanation && <p className="offer-explanation">{offer.explanation}</p>}
      <ErrorNote error={act.error} />
      {canAccept && (
        <div className="form-actions">
          <button type="button" className="btn btn-primary" disabled={act.busy} onClick={() => void post("accept")}>
            Принять предложение
          </button>
          {app.allowed_actions.includes("DECLINE_OFFER") &&
            (confirmDecline ? (
              <span className="confirm">
                Заявка будет закрыта.
                <button type="button" className="btn btn-danger btn-sm" disabled={act.busy} onClick={() => void post("decline")}>
                  Да, отказаться
                </button>
                <button type="button" className="btn btn-quiet btn-sm" onClick={() => setConfirmDecline(false)}>
                  Нет
                </button>
              </span>
            ) : (
              <button type="button" className="btn btn-quiet" onClick={() => setConfirmDecline(true)}>
                Отказаться
              </button>
            ))}
        </div>
      )}
      {canSign && (
        <div className="sign">
          <h3>Подписание договора</h3>
          <p>
            В прототипе подпись симулируется: настоящая электронная подпись не создаётся, деньги не переводятся. После подписи банк
            проведёт демонстрационную выдачу.
          </p>
          <label className="check">
            <input type="checkbox" checked={simulated} onChange={(e) => setSimulated(e.target.checked)} />
            Понимаю, что это демонстрационная подпись
          </label>
          <button type="button" className="btn btn-primary" disabled={!simulated || act.busy} onClick={() => void post("sign")}>
            Подписать договор
          </button>
        </div>
      )}
    </div>
  );
}

function CancelButton({ app, reload }: { app: ClientApplication; reload: () => Promise<void> }) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const act = useAction();
  const intent = useIntent();
  if (!open) {
    return (
      <button type="button" className="btn btn-quiet cancel-link" onClick={() => setOpen(true)}>
        Отозвать заявку
      </button>
    );
  }
  return (
    <div className="cancel-box">
      <Field label="Почему отзываете? Необязательно" htmlFor="cancel-reason">
        <textarea id="cancel-reason" rows={2} maxLength={500} value={reason} onChange={(e) => setReason(e.target.value)} />
      </Field>
      <ErrorNote error={act.error} />
      <div className="form-actions">
        <button
          type="button"
          className="btn btn-danger"
          disabled={act.busy}
          onClick={() =>
            void act.run(async () => {
              await intent.send((key) =>
                call(
                  clientApi.POST("/client/applications/{application_id}/cancel", {
                    params: { path: { application_id: app.id }, header: { "Idempotency-Key": key } },
                    body: reason.trim() ? { reason: reason.trim() } : {},
                  }),
                ),
              );
              await reload();
            })
          }
        >
          Отозвать
        </button>
        <button type="button" className="btn btn-quiet" onClick={() => setOpen(false)}>
          Не отзывать
        </button>
      </div>
    </div>
  );
}

function Summary({ app }: { app: ClientApplication }) {
  return (
    <details className="panel summary">
      <summary>Данные заявки</summary>
      <Dl
        items={[
          ["Сумма", money(app.form?.amount)],
          ["Срок", months(app.form?.term_months)],
          ["Цель", app.form?.purpose ? purposeLabel[app.form.purpose] : "—"],
          ["На что потратить", app.form?.purpose_description || app.purpose_description || "—"],
          ["Документы", app.documents.length ? app.documents.map((d) => d.file_name).join(", ") : "нет"],
          ["Создана", dateTime(app.created_at)],
        ]}
      />
      <CompanySnapshot company={app.company} />
    </details>
  );
}

function Timeline({ id, version }: { id: string; version: number }) {
  const [items, setItems] = useState<TimelineEntry[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  useEffect(() => {
    call(clientApi.GET("/client/applications/{application_id}/timeline", { params: { path: { application_id: id } } }))
      .then((r) => {
        setItems(r.items);
        setError(null);
      })
      .catch(setError);
  }, [id, version]);
  // The timeline endpoint may not be deployed yet; the rest of the card works without it.
  if (error instanceof ApiError && error.status === 501) return null;
  return (
    <section className="timeline-wrap">
      <h2>История</h2>
      <ErrorNote error={error} />
      {items && (
        <ol className="timeline">
          {[...items].reverse().map((e) => (
            <li key={e.id}>
              <time dateTime={e.occurred_at}>{dateTime(e.occurred_at)}</time>
              <div>
                <strong>{e.message || clientStatusLabel[e.status]}</strong>
                {e.message && <p>{clientStatusLabel[e.status]}</p>}
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
