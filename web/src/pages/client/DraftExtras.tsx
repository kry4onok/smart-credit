import type { AssessmentPreferences, DocumentMetadata, LegalForm, Requirement, RequirementsPlan } from "../../api/types";
import { DocumentSlot, MoneyInput } from "../../components/inputs";
import { Choices, Field } from "../../components/ui";
import type { Hint } from "../../lib/hints";
import { planReasonLabel, priorityLabel, requiredAtLabel } from "../../lib/labels";

const levelText: Record<Hint["level"], string> = {
  good: "помогает",
  warn: "может снизить шансы",
  bad: "мешает",
  info: "нужны данные",
};

// HintsPanel turns the entered numbers into plain-language early warnings with one-click fixes.
export function HintsPanel({ hints, onFix }: { hints: Hint[]; onFix: (fix: NonNullable<Hint["fix"]>) => void }) {
  if (!hints.length) return null;
  const worst = hints[0].level;
  return (
    <section className={`hints hints-${worst}`} aria-labelledby="hints-title">
      <div className="hints-head">
        <h3 id="hints-title">Что может повлиять на решение</h3>
        <span className="hints-note">Подсказка по вашим цифрам. Решение принимает банк.</span>
      </div>
      <ul>
        {hints.map((h) => (
          <li key={h.id} className={`hint hint-${h.level}`}>
            <span className="hint-dot" aria-hidden />
            <div className="hint-body">
              <strong>
                {h.title} <span className="hint-level">— {levelText[h.level]}</span>
              </strong>
              <p>{h.text}</p>
              {h.meter && (
                <span className="hint-meter" aria-hidden>
                  <span className="hint-meter-mark" style={{ left: `${h.meter.good * 100}%` }} />
                  <span className="hint-meter-fill" style={{ width: `${h.meter.value * 100}%` }} />
                  <span className="hint-meter-label">{h.meter.label}</span>
                </span>
              )}
            </div>
            {h.fix && (
              <button type="button" className="btn btn-secondary btn-sm" onClick={() => onFix(h.fix!)}>
                {h.fix.label}
              </button>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

// PreferencesSection: what matters to the client if the bank cannot give exactly what was asked.
export function PreferencesSection({ prefs, onChange, amountKopecks }: { prefs: AssessmentPreferences; onChange: (p: AssessmentPreferences) => void; amountKopecks: number | undefined }) {
  type Priority = "KEEP_AMOUNT" | "LOW_PAYMENT" | "LOW_TOTAL_COST";
  const priority = (prefs.priority || "KEEP_AMOUNT") as Priority;
  const set = (patch: Partial<AssessmentPreferences>) => {
    const next = { ...prefs, ...patch };
    for (const key of Object.keys(next) as (keyof AssessmentPreferences)[]) if (next[key] === null || next[key] === undefined) delete next[key];
    onChange(next);
  };
  return (
    <div className="prefs">
      <p className="field-hint">
        Если банк не сможет дать ровно запрошенное, он подберёт до трёх вариантов под эти пожелания. Поля необязательные.
      </p>
      <Field label="Что для вас важнее" group>
        <Choices
          name="priority"
          className="outcomes prefs-priority"
          value={priority}
          onChange={(v) => set({ priority: v })}
          options={(["KEEP_AMOUNT", "LOW_PAYMENT", "LOW_TOTAL_COST"] as Priority[]).map((k) => ({ value: k, label: priorityLabel[k].title, hint: priorityLabel[k].hint }))}
          optionClassName={(_, on) => `outcome-opt${on ? " on" : ""}`}
        />
      </Field>
      <div className="form-grid">
        <Field label="Комфортный платёж в месяц, до" htmlFor="pref-payment" hint="Банк не предложит вариант с платежом выше">
          <MoneyInput id="pref-payment" value={prefs.max_payment_kopecks ?? undefined} onChange={(v) => set({ max_payment_kopecks: v ?? undefined })} />
        </Field>
        <Field label="Минимальная сумма, которая решит задачу" htmlFor="pref-min" hint="Без неё банк не станет уменьшать сумму">
          <MoneyInput id="pref-min" value={prefs.minimum_useful_amount_kopecks ?? undefined} onChange={(v) => set({ minimum_useful_amount_kopecks: v ?? undefined })} />
        </Field>
        <Field label="Срок не больше, месяцев" htmlFor="pref-term">
          <input
            id="pref-term"
            type="number"
            min={1}
            value={prefs.max_term_months ?? ""}
            onChange={(e) => {
              const n = e.target.value === "" ? undefined : Number(e.target.value);
              set({ max_term_months: n && Number.isSafeInteger(n) && n > 0 ? n : undefined });
            }}
          />
        </Field>
      </div>
      {prefs.minimum_useful_amount_kopecks && amountKopecks && prefs.minimum_useful_amount_kopecks > amountKopecks && (
        <p className="field-error">Минимальная сумма больше запрошенной — уменьшите её.</p>
      )}
    </div>
  );
}

const stageOrder = ["SUBMIT", "ASSESSMENT", "ON_REQUEST"] as const;

// RequirementsSection shows the bank's plan for this draft: what is needed now, what later, and why.
export function RequirementsSection({
  applicationId,
  plan,
  legalForm,
  documents,
  canUpload,
  onUploaded,
}: {
  applicationId: string;
  plan: RequirementsPlan;
  legalForm: LegalForm | undefined;
  documents: DocumentMetadata[];
  canUpload: boolean;
  onUploaded: (doc: DocumentMetadata) => void;
}) {
  const documentsNeeded = plan.items.filter((r) => r.kind === "DOCUMENT");
  const fields = plan.items.filter((r) => r.kind === "FIELD");
  const missingFields = fields.filter((r) => r.status === "MISSING");
  return (
    <div className="requirements" id="requirements">
      {plan.route === "MANUAL_REVIEW" && (
        <div className="note note-attention">
          Заявку проверит специалист банка{plan.reason_codes.length ? ": " + plan.reason_codes.map((c) => planReasonLabel[c] ?? c).join("; ") : ""}. Это не
          отказ — просто решение займёт больше времени.
        </div>
      )}
      {missingFields.length > 0 && (
        <p className="field-hint" role="status">
          Не заполнено: {missingFields.map((r) => r.label.toLowerCase()).join(", ")}.
        </p>
      )}
      {documentsNeeded.length === 0 && <p className="field-hint">Документы сейчас не нужны: банк сначала проверит данные в своих источниках.</p>}
      {stageOrder.map((stage) => {
        const items = documentsNeeded.filter((r) => r.required_at === stage);
        if (!items.length) return null;
        return (
          <div key={stage} className="req-group">
            <h4>
              {requiredAtLabel[stage]}
              {stage !== "SUBMIT" && <small> — можно приложить сейчас, чтобы ускорить рассмотрение</small>}
            </h4>
            {items.map((r: Requirement) => (
              <DocumentSlot key={r.id} applicationId={applicationId} requirement={r} legalForm={legalForm} documents={documents} enabled={canUpload} onUploaded={onUploaded} />
            ))}
          </div>
        );
      })}
    </div>
  );
}
