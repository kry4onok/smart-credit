import { cloneElement, Component, isValidElement, useEffect, useState, type ReactNode } from "react";
import { errorMessage } from "../api/problem";
import type { Tone } from "../lib/labels";

export function Badge({ tone, children }: { tone: Tone; children: ReactNode }) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}

export function ErrorNote({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  if (!error) return null;
  return (
    <div className="note note-danger" role="alert">
      <span>{errorMessage(error)}</span>
      {onRetry && (
        <button type="button" className="btn btn-quiet btn-sm" onClick={onRetry}>
          Повторить
        </button>
      )}
    </div>
  );
}

export function Loading({ label = "Загружаем…" }: { label?: string }) {
  return (
    <div className="loading" role="status">
      <span className="spinner" aria-hidden />
      {label}
    </div>
  );
}

// Field labels one control and links its hint or error through aria-describedby.
// With group, it labels a set of controls (radios, checkboxes) as fieldset + legend.
export function Field({
  label,
  hint,
  error,
  children,
  htmlFor,
  group,
}: {
  label: string;
  hint?: ReactNode;
  error?: string | null;
  children: ReactNode;
  htmlFor?: string;
  group?: boolean;
}) {
  const noteId = htmlFor ? `${htmlFor}-note` : undefined;
  const note = error ? (
    <p id={noteId} className="field-error">
      {error}
    </p>
  ) : hint ? (
    <p id={noteId} className="field-hint">
      {hint}
    </p>
  ) : null;
  const control =
    note && noteId && isValidElement<{ "aria-describedby"?: string }>(children)
      ? cloneElement(children, { "aria-describedby": noteId })
      : children;
  if (group) {
    return (
      <fieldset className={`field group${error ? " field-invalid" : ""}`}>
        <legend>{label}</legend>
        {control}
        {note}
      </fieldset>
    );
  }
  return (
    <div className={`field${error ? " field-invalid" : ""}`}>
      <label htmlFor={htmlFor}>{label}</label>
      {control}
      {note}
    </div>
  );
}

type ChoiceProps<T extends string> = {
  name: string;
  options: { value: T; label: ReactNode; hint?: ReactNode }[];
  value: T | null | undefined;
  onChange: (value: T) => void;
  className: string;
  optionClassName: (value: T, checked: boolean) => string;
};

// Choices renders native radio inputs styled as cards: arrows, Tab and screen readers work as usual.
export function Choices<T extends string>({ name, options, value, onChange, className, optionClassName }: ChoiceProps<T>) {
  return (
    <div className={className}>
      {options.map((o) => (
        <label key={o.value} className={`choice ${optionClassName(o.value, value === o.value)}`}>
          <input type="radio" name={name} value={o.value} checked={value === o.value} onChange={() => onChange(o.value)} />
          {typeof o.label === "string" ? <strong>{o.label}</strong> : o.label}
          {o.hint && <span>{o.hint}</span>}
        </label>
      ))}
    </div>
  );
}

// Keeps the browser tab title in sync with the page, so history and screen readers name the page.
export function useTitle(title: string | undefined) {
  useEffect(() => {
    if (title) document.title = `${title} — Кредитный кабинет`;
  }, [title]);
}

type BoundaryState = { error: Error | null };

// ErrorBoundary replaces a crashed screen with a way back instead of a blank page.
export class ErrorBoundary extends Component<{ children: ReactNode }, BoundaryState> {
  state: BoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): BoundaryState {
    return { error };
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="crash panel" role="alert">
        <h1>Экран не открылся</h1>
        <p>Ваши данные сохранены на сервере. Обновите страницу; если ошибка повторится, вернитесь на главную.</p>
        <div className="form-actions">
          <button type="button" className="btn btn-primary" onClick={() => window.location.reload()}>
            Обновить страницу
          </button>
          <a className="btn btn-secondary" href="/">
            На главную
          </a>
        </div>
      </div>
    );
  }
}

export function Dl({ items }: { items: [string, ReactNode][] }) {
  return (
    <dl className="dl">
      {items.map(([term, value]) => (
        <div key={term}>
          <dt>{term}</dt>
          <dd>{value}</dd>
        </div>
      ))}
    </dl>
  );
}

// Tracks an async action: busy flag and the last error, shown next to the button that started it.
export function useAction() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  async function run<T>(fn: () => Promise<T>): Promise<T | undefined> {
    setBusy(true);
    setError(null);
    try {
      return await fn();
    } catch (e) {
      setError(e);
      return undefined;
    } finally {
      setBusy(false);
    }
  }
  return { busy, error, setError, run };
}

// Re-renders every intervalMs so relative times («ещё 3 ч») stay current.
export function useNow(intervalMs = 30000) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
}
