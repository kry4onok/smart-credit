import { useEffect, useRef } from "react";

// crypto.randomUUID exists only in secure contexts; a demo may be opened over plain HTTP.
export function uuid(): string {
  if (typeof crypto.randomUUID === "function") return crypto.randomUUID();
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  const h = [...b].map((x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

export type Intent = { send: <T>(fn: (key: string) => Promise<T>) => Promise<T> };

// One Idempotency-Key per user intent. A retry after a network failure or a 5xx repeats the key,
// so the server returns the first result instead of acting twice; a definite 4xx or success starts over.
export function makeIntent(): Intent {
  let key: string | null = null;
  return {
    async send<T>(fn: (key: string) => Promise<T>) {
      key ??= uuid();
      try {
        const result = await fn(key);
        key = null;
        return result;
      } catch (e) {
        const status = (e as { status?: number }).status ?? 0;
        if (status >= 400 && status < 500) key = null;
        throw e;
      }
    },
  };
}

export function useIntent(): Intent {
  const ref = useRef<Intent | null>(null);
  ref.current ??= makeIntent();
  return ref.current;
}

// Separate intents per target (for example per product), so switching the target never reuses a key.
export function useIntents(): (target: string) => Intent {
  const ref = useRef(new Map<string, Intent>());
  return (target) => {
    let intent = ref.current.get(target);
    if (!intent) ref.current.set(target, (intent = makeIntent()));
    return intent;
  };
}

export function etag(version: number): string {
  return `"${version}"`;
}

// Checks the INN control digits (10 digits for a company, 12 for a sole proprietor).
export function isValidInn(inn: string): boolean {
  const d = inn.split("").map(Number);
  const check = (weights: number[]) => (weights.reduce((sum, w, i) => sum + w * d[i], 0) % 11) % 10;
  if (/^\d{10}$/.test(inn)) return check([2, 4, 10, 3, 5, 9, 4, 6, 8]) === d[9];
  if (/^\d{12}$/.test(inn)) {
    return check([7, 2, 4, 10, 3, 5, 9, 4, 6, 8]) === d[10] && check([3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8]) === d[11];
  }
  return false;
}

// Calls fn every intervalMs while enabled and the tab is visible. Status updates use polling by design.
// When fn returns a promise, a tick is skipped until it settles, so slow answers do not pile up.
export function usePolling(fn: () => unknown, intervalMs: number, enabled: boolean) {
  const saved = useRef(fn);
  saved.current = fn;
  useEffect(() => {
    if (!enabled) return;
    let inFlight = false;
    const id = window.setInterval(() => {
      if (inFlight || document.visibilityState !== "visible") return;
      const result = saved.current();
      if (result instanceof Promise) {
        inFlight = true;
        result.catch(() => undefined).finally(() => (inFlight = false));
      }
    }, intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs, enabled]);
}

// Polling refreshes only the first page. Once the user loaded more with «Показать ещё»,
// fresh rows replace their old copies and the extra pages stay in place.
export function mergeFirstPage<T extends { id: string }>(prev: T[] | null, first: T[]): T[] {
  const fresh = new Set(first.map((i) => i.id));
  return [...first, ...(prev ?? []).filter((i) => !fresh.has(i.id))];
}

// Appends the next page, skipping rows already shown (a row can move between pages while polling).
export function appendPage<T extends { id: string }>(prev: T[] | null, next: T[]): T[] {
  const seen = new Set((prev ?? []).map((i) => i.id));
  return [...(prev ?? []), ...next.filter((i) => !seen.has(i.id))];
}
