import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { bankApi } from "../../api/http";
import { call } from "../../api/problem";
import type { BankStatus, CaseListItem } from "../../api/types";
import { useSession } from "../../auth/session";
import { Badge, ErrorNote, Loading, useAction, useTitle } from "../../components/ui";
import { age, money, caseNumber, shortId } from "../../lib/format";
import { bankStatusLabel, bankStatusTone } from "../../lib/labels";
import { appendPage, mergeFirstPage, usePolling } from "../../lib/util";
import { QueueSummary, isStale } from "./QueueSummary";

const statuses = Object.keys(bankStatusLabel) as BankStatus[];

export function QueuePage() {
  const { user } = useSession();
  const [params, setParams] = useSearchParams();
  const status = (params.get("status") as BankStatus | null) ?? undefined;
  const assignee = params.get("assignee") ?? undefined;
  // «Ждут больше суток»: the API has no age filter, so the loaded cases are narrowed here.
  const stale = params.get("waiting") === "day";
  const [items, setItems] = useState<CaseListItem[] | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  // The summary covers the whole queue regardless of the table filters.
  const [all, setAll] = useState<{ items: CaseListItem[]; partial: boolean } | null>(null);
  useTitle("Очередь дел");

  const extended = useRef(false);
  const pager = useAction();
  const query = { limit: 50, ...(status ? { status } : {}), ...(assignee ? { assignee } : {}) };

  const load = useCallback(async () => {
    try {
      const [page, overview] = await Promise.all([
        call(bankApi.GET("/backoffice/applications", { params: { query } })),
        call(bankApi.GET("/backoffice/applications", { params: { query: { limit: 100 } } })),
      ]);
      setItems((prev) => (extended.current ? mergeFirstPage(prev, page.items) : page.items));
      if (!extended.current) setCursor(page.next_cursor ?? null);
      setAll({ items: overview.items, partial: !!overview.next_cursor });
      setError(null);
    } catch (e) {
      setError(e);
    }
  }, [status, assignee]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    extended.current = false;
    setItems(null);
    void load();
  }, [load]);
  usePolling(load, 8000, true);

  function more() {
    if (!cursor || pager.busy) return;
    void pager.run(async () => {
      const page = await call(bankApi.GET("/backoffice/applications", { params: { query: { ...query, cursor } } }));
      extended.current = true;
      setItems((prev) => appendPage(prev, page.items));
      setCursor(page.next_cursor ?? null);
    });
  }

  function filter(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    // The «больше суток» selection only makes sense for manual review.
    if (key === "status") next.delete("waiting");
    setParams(next);
  }

  const rows = items && stale ? items.filter((c) => isStale(c)).sort((a, b) => a.created_at.localeCompare(b.created_at)) : items;

  return (
    <>
      <div className="page-head">
        <h1>Очередь дел</h1>
      </div>
      {all && (
        <QueueSummary
          items={all.items}
          partial={all.partial}
          me={user?.id}
          onFilter={(f) => setParams({ ...(f.status ? { status: f.status } : {}), ...(f.assignee ? { assignee: f.assignee } : {}), ...(f.stale ? { waiting: "day" } : {}) })}
        />
      )}
      <div className="filters">
        <label>
          Статус
          <select value={status ?? ""} onChange={(e) => filter("status", e.target.value)}>
            <option value="">Все</option>
            {statuses.map((s) => (
              <option key={s} value={s}>
                {bankStatusLabel[s]}
              </option>
            ))}
          </select>
        </label>
        <label>
          Исполнитель
          <select value={assignee ?? ""} onChange={(e) => filter("assignee", e.target.value)}>
            <option value="">Все</option>
            <option value="me">Мои</option>
            <option value="unassigned">Свободные</option>
          </select>
        </label>
      </div>
      {stale && (
        <p className="note note-attention">
          Показаны дела на ручном разборе, которые ждут больше суток, — сначала самые старые.
          <button type="button" className="btn btn-quiet btn-sm" onClick={() => filter("waiting", "")}>
            Показать все
          </button>
        </p>
      )}
      <ErrorNote error={error} onRetry={load} />
      <ErrorNote error={pager.error} onRetry={more} />
      {!items && !error && <Loading />}
      {rows && rows.length === 0 && <p className="empty panel">Под эти фильтры дел нет.</p>}
      {rows && rows.length > 0 && (
        <div className="table-wrap panel">
          <table className="cases">
            <thead>
              <tr>
                <th>Компания</th>
                <th className="num">Сумма</th>
                <th>Этап</th>
                <th>Исполнитель</th>
                <th className="num">В работе</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((c) => (
                <tr key={c.id} className={c.status === "MANUAL_REVIEW" ? "row-manual" : ""}>
                  <td>
                    <Link to={`/bank/cases/${c.id}`}>{c.company_name ?? `Дело ${shortId(c.id)}`}</Link>
                    <small>№ {caseNumber(c.id)}</small>
                  </td>
                  <td className="num">{money(c.requested_amount)}</td>
                  <td>
                    <Badge tone={bankStatusTone[c.status]}>{bankStatusLabel[c.status]}</Badge>
                  </td>
                  <td>{!c.assignee_id ? <span className="muted">свободно</span> : c.assignee_id === user?.id ? "вы" : "другой аналитик"}</td>
                  <td className="num">{age(c.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {cursor && (
        <button type="button" className="btn btn-secondary" disabled={pager.busy} onClick={more}>
          {pager.busy ? "Загружаем…" : "Показать ещё"}
        </button>
      )}
    </>
  );
}
