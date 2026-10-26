import { useState } from "react";
import { assessmentApi } from "../api/http";
import { ApiError, call } from "../api/problem";
import type { HelpContext, HelpResponse, HelpSelectionRequest } from "../api/types";
import { tidyBlocks } from "../lib/helpBlocks";
import { ErrorNote, useAction } from "./ui";

type Mode = HelpSelectionRequest["mode"];

export const clientModes: { mode: Mode; label: string }[] = [
  { mode: "NEXT_STEP", label: "Что делать дальше" },
  { mode: "REQUIREMENT", label: "Зачем эти документы" },
  { mode: "ASSESSMENT", label: "Как банк оценил заявку" },
  { mode: "COMPARE_OPTIONS", label: "Сравнить варианты" },
  { mode: "DECISION", label: "Почему такое решение" },
];

export const analystModes: { mode: Mode; label: string }[] = [
  { mode: "ASSESSMENT", label: "Разбор оценки" },
  { mode: "COMPARE_OPTIONS", label: "Сравнить варианты" },
  { mode: "CHANGES", label: "Что изменилось" },
  { mode: "NEXT_STEP", label: "Следующий шаг" },
];


// Navigation targets for assistant actions. Actions only move the view, never run a bank command.
const actionAnchor: Record<string, string> = {
  ACTION_KIND_OPEN_REQUIREMENT: "requirements",
  ACTION_KIND_OPEN_FACTOR: "assessment",
  ACTION_KIND_OPEN_OPTIONS: "options",
  ACTION_KIND_OPEN_OFFER: "offer",
  ACTION_KIND_OPEN_TIMELINE: "timeline",
};

// Some modes need the objects they talk about: options to compare, requirements to explain.
// Target counts follow the server's Validate: a mode without enough targets is answered with
// a plain note instead of a request the server rejects with 400.
const modeTargets: Partial<Record<Mode, { kind: string; min: number; max: number; missing: string }>> = {
  COMPARE_OPTIONS: { kind: "TARGET_KIND_OPTION", min: 2, max: 3, missing: "Для сравнения нужно хотя бы два рассчитанных варианта." },
  REQUIREMENT: { kind: "TARGET_KIND_REQUIREMENT", min: 1, max: 1, missing: "Сейчас банк не запрашивает документов." },
  CHANGES: { kind: "TARGET_KIND_CHANGE", min: 1, max: 1, missing: "С прошлого расчёта ничего не изменилось." },
};

type Props = {
  applicationId: string;
  audience: "client" | "analyst";
  modes: { mode: Mode; label: string }[];
  onOpenProfile?: () => void;
};

// HelpPanel asks the backend assistant to explain facts the bank has already established.
// The backend builds the authorized context; the model only picks phrases from a fixed catalog,
// and a deterministic template answers when the model is unavailable.
export function HelpPanel({ applicationId, audience, modes, onOpenProfile }: Props) {
  const [answer, setAnswer] = useState<{ mode: Mode; response: HelpResponse } | null>(null);
  const [question, setQuestion] = useState("");
  const act = useAction();
  const path = audience === "client" ? "/client/applications/{application_id}/help" : "/backoffice/applications/{application_id}/help";

  async function context(): Promise<HelpContext> {
    return call(assessmentApi.GET(path, { params: { path: { application_id: applicationId } } }));
  }

  async function ask(mode: Mode, text?: string) {
    const response = await act.run(async () => {
      const send = async () => {
        const c = await context();
        const rule = modeTargets[mode];
        const targets = rule ? (c.targets ?? []).filter((t) => t.kind === rule.kind && t.id).slice(0, rule.max).map((t) => t.id ?? "") : [];
        if (rule && targets.length < rule.min) throw new ApiError(0, rule.missing);
        return call(
          assessmentApi.POST(path, {
            params: { path: { application_id: applicationId } },
            body: {
              context_id: c.context_id ?? "",
              source_revision: c.source_revision ?? "",
              catalog_version: (c.catalog_version ?? "smart-credit-help-v1") as "smart-credit-help-v1",
              mode,
              detail: "DETAILED",
              target_ids: targets,
              ...(text ? { question: text } : {}),
            },
          }),
        );
      };
      try {
        return await send();
      } catch (e) {
        // The context lives about a minute and changes with every status update: refetch once.
        if (e instanceof ApiError && e.code === "CONTEXT_STALE") return send();
        throw e;
      }
    });
    if (response) setAnswer({ mode, response });
  }

  function open(kind: string | undefined) {
    if (kind === "ACTION_KIND_OPEN_PROFILE") return onOpenProfile?.();
    const anchor = kind ? actionAnchor[kind] : undefined;
    if (anchor) document.getElementById(anchor)?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  const response = answer?.response;
  // Several targets of one kind produce identical buttons that lead to the same place: keep one.
  const actions = (response?.actions ?? []).filter((a, i, all) => all.findIndex((b) => b.kind === a.kind && b.label === a.label) === i);
  const fromModel = response?.source === "RESPONSE_SOURCE_MODEL";
  const fallbackLabel: Record<string, string> = {
    FALLBACK_REASON_MODEL_TIMEOUT: "Ответ из проверенных данных: модель не успела ответить",
    FALLBACK_REASON_MODEL_UNAVAILABLE: "Ответ из проверенных данных: модель недоступна",
    FALLBACK_REASON_QUEUE_FULL: "Ответ из проверенных данных: модель занята",
    FALLBACK_REASON_INVALID_MODEL_OUTPUT: "Ответ из проверенных данных: ответ модели не прошёл проверку",
    FALLBACK_REASON_NO_SUPPORTED_CONTEXT: "Ответ из проверенных данных: пока не хватает сведений",
  };
  const sourceLabel = fromModel
    ? "Модель выбрала подходящие проверенные сведения банка"
    : fallbackLabel[response?.fallback_reason ?? ""] ?? "Ответ из проверенных данных банка";
  return (
    <section className="panel helper" aria-labelledby={`helper-${audience}`}>
      <div className="helper-head">
        <h2 id={`helper-${audience}`}>Помощник</h2>
        <p className="muted">Объясняет уже рассчитанные банком данные. Не меняет решение и условия.</p>
      </div>
      <div className="helper-modes" role="group" aria-label="Что объяснить">
        {modes.map((m) => (
          <button key={m.mode} type="button" className={`chip${answer?.mode === m.mode ? " chip-on" : ""}`} disabled={act.busy} onClick={() => void ask(m.mode)}>
            {m.label}
          </button>
        ))}
      </div>
      <form
        className="helper-question"
        onSubmit={(e) => {
          e.preventDefault();
          if (question.trim()) void ask("QUESTION", question.trim());
        }}
      >
        <label className="visually-hidden" htmlFor={`helper-q-${audience}`}>
          Свой вопрос
        </label>
        <input id={`helper-q-${audience}`} placeholder={audience === "client" ? "Спросите о своей заявке, например: почему сумма меньше?" : "Вопрос по делу, например: что сильнее всего снижает сумму?"} maxLength={300} value={question} onChange={(e) => setQuestion(e.target.value)} />
        <button className="btn btn-secondary btn-sm" disabled={act.busy || !question.trim()}>
          Спросить
        </button>
      </form>
      <ErrorNote error={act.error} />
      {act.busy && (
        <p className="pending" role="status">
          <span className="spinner" aria-hidden />
          Помощник готовит ответ…
        </p>
      )}
      {response && !act.busy && (
        <div className="helper-answer" aria-live="polite">
          {tidyBlocks(response.blocks ?? []).map(({ block, title }, i) => (
            <div key={i} className="helper-block">
              {title && <h3>{title}</h3>}
              {(block.items ?? []).map((item, j) => (
                <p key={item.statement_id ?? j}>{item.text}</p>
              ))}
            </div>
          ))}
          {(response.blocks ?? []).length === 0 && <p className="muted">По этому вопросу у банка пока нет данных для объяснения.</p>}
          {actions.length > 0 && (
            <div className="helper-actions">
              {actions.map((a) => (
                <button key={a.id} type="button" className="btn btn-quiet btn-sm" onClick={() => open(a.kind)}>
                  {a.label}
                </button>
              ))}
            </div>
          )}
          <p className="helper-source">
            {sourceLabel}
          </p>
        </div>
      )}
    </section>
  );
}
