import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { clientApi } from "../../api/http";
import { call } from "../../api/problem";
import type { ApplicationListItem } from "../../api/types";
import { MiniTrack } from "../../components/Pipeline";
import { Badge, ErrorNote, Loading, useAction, useTitle } from "../../components/ui";
import { date, moneyShort } from "../../lib/format";
import { clientStatusLabel, clientStatusTone } from "../../lib/labels";
import { appendPage, mergeFirstPage, usePolling } from "../../lib/util";
import { useProductNames } from "./products";

const settled = new Set(["DRAFT", "DISBURSED", "REJECTED", "CLOSED"]);
// Statuses where the bank waits for the client: shown first.
const yourMove = new Set(["NEEDS_INFO", "OFFER_AVAILABLE", "FORMALIZING"]);

export function ApplicationsPage() {
  const [items, setItems] = useState<ApplicationListItem[] | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const products = useProductNames();
  useTitle("Мои заявки");

  const extended = useRef(false);
  const pager = useAction();

  const load = useCallback(async () => {
    try {
      const page = await call(clientApi.GET("/client/applications", { params: { query: { limit: 50 } } }));
      setItems((prev) => (extended.current ? mergeFirstPage(prev, page.items) : page.items));
      if (!extended.current) setCursor(page.next_cursor ?? null);
      setError(null);
    } catch (e) {
      setError(e);
    }
  }, []);

  useEffect(() => void load(), [load]);
  usePolling(load, 5000, !!items?.some((i) => !settled.has(i.status)));

  function more() {
    if (!cursor || pager.busy) return;
    void pager.run(async () => {
      const page = await call(clientApi.GET("/client/applications", { params: { query: { limit: 50, cursor } } }));
      extended.current = true;
      setItems((prev) => appendPage(prev, page.items));
      setCursor(page.next_cursor ?? null);
    });
  }

  return (
    <>
      <div className="page-head">
        <h1>Мои заявки</h1>
        <Link to="/cabinet/products" className="btn btn-primary">
          Новая заявка
        </Link>
      </div>
      <ErrorNote error={error} onRetry={load} />
      <ErrorNote error={pager.error} onRetry={more} />
      {!items && !error && <Loading />}
      {items && items.length === 0 && (
        <div className="empty panel">
          <h2>Заявок пока нет</h2>
          <p>Расскажите, на что нужны деньги, — покажем подходящие кредиты и их условия.</p>
          <Link to="/cabinet/products" className="btn btn-primary">
            Подобрать кредит
          </Link>
        </div>
      )}
      {items && items.length > 0 && (
        <>
          {[
            { title: "Нужно ваше действие", rows: items.filter((i) => yourMove.has(i.status)), urgent: true },
            { title: "Остальные заявки", rows: items.filter((i) => !yourMove.has(i.status)), urgent: false },
          ]
            .filter((group) => group.rows.length)
            .map((group, _, groups) => (
              <section key={group.title} className="app-group">
                {groups.length > 1 && <h2>{group.title}</h2>}
                <ul className="app-list">
                  {group.rows.map((item) => (
                    <li key={item.id}>
                      <Link to={`/cabinet/applications/${item.id}`} className={`app-row${group.urgent ? " app-row-urgent" : ""}`}>
                        <span className="app-row-amount">{moneyShort(item.amount)}</span>
                        <span className="app-row-main">
                          <strong>{products[item.product_id]?.name ?? "Кредит"}</strong>
                          <span>{item.next_action}</span>
                        </span>
                        <span className="app-row-status">
                          <Badge tone={clientStatusTone[item.status]}>{clientStatusLabel[item.status]}</Badge>
                          <MiniTrack status={item.status} />
                        </span>
                        <span className="app-row-date">{date(item.created_at)}</span>
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            ))}
        </>
      )}
      {cursor && (
        <button type="button" className="btn btn-secondary" disabled={pager.busy} onClick={more}>
          {pager.busy ? "Загружаем…" : "Показать ещё"}
        </button>
      )}
    </>
  );
}
