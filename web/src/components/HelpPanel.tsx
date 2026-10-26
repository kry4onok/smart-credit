import { useState } from "react";
import { assessmentApi } from "../api/http";
import { ApiError, call } from "../api/problem";
import type { HelpContext, HelpResponse, HelpSelectionRequest } from "../api/types";
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

const blockTitle: Record<string, string> = {
  BLOCK_KIND_DETAILS: "Подробнее",
  BLOCK_KIND_LIMITATIONS: "Ограничения",
  BLOCK_KIND_NEXT_STEPS: "Что сделать",
  BLOCK_KIND_CHANGES: "Что изменилось",
  BLOCK_KIND_COMPARISON: "Сравнение",
};

// Navigation targets for assistant actions. Actions only move the view, never run a bank command.
const actionAnchor: Record<string, string> = {
  ACTION_KIND_OPEN_REQUIREMENT: "requirements",
  ACTION_KIND_OPEN_FACTOR: "assessment",
  ACTION_KIND_OPEN_OPTIONS: "options",
  ACTION_KIND_OPEN_OFFER: "offer",
  ACTION_KIND_OPEN_TIMELINE: "timeline",
};

// Some modes need the objects they talk about: options to compare, requirements to explain.
const modeTargets: Partial<Record<Mode, string>> = {
  COMPARE_OPTIONS: "TARGET_KIND_OPTION",
  REQUIREMENT: "TARGET_KIND_REQUIREMENT",
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
        return call(
          assessmentApi.POST(path, {
            params: { path: { application_id: applicationId } },
            body: {
              context_id: c.context_id ?? "",
              source_revision: c.source_revision ?? "",
              catalog_version: (c.catalog_version ?? "smart-credit-help-v1") as "smart-credit-help-v1",
              mode,
              detail: "DETAILED",
              target_ids: modeTargets[mode] ? (c.targets ?? []).filter((t) => t.kind === modeTargets[mode]).slice(0, 3).map((t) => t.id ?? "").filter(Boolean) : [],
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
          {(response.blocks ?? []).map((block, i) => (
            <div key={i} className="helper-block">
              {block.kind && blockTitle[block.kind] && <h3>{blockTitle[block.kind]}</h3>}
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
            {fromModel ? `Сформулировано моделью ${response.model_version ?? ""} по данным банка` : "Ответ по шаблону: модель сейчас недоступна, факты те же"}
          </p>
        </div>
      )}
    </section>
  );
}
