import type { BankStatus, CaseListItem } from "../../api/types";

type Group = { key: string; label: string; color: string; statuses: BankStatus[] };

// Order and colours were checked with the dataviz palette validator (CVD-safe with labels and gaps).
const groups: Group[] = [
  { key: "auto", label: "Автоматическая обработка", color: "#2c58b5", statuses: ["SUBMITTED", "VALIDATION", "ENRICHMENT", "SCORING", "DECISION"] },
  { key: "manual", label: "Ручной разбор", color: "#a35700", statuses: ["MANUAL_REVIEW"] },
  { key: "client", label: "Ждут клиента", color: "#7a4fb5", statuses: ["NEEDS_INFO", "OFFER", "COUNTER_OFFER"] },
  { key: "signing", label: "Оформление", color: "#4c697b", statuses: ["OFFER_ACCEPTED", "SIGNING"] },
  { key: "done", label: "Выдано", color: "#0f7451", statuses: ["DISBURSED"] },
  { key: "closed", label: "Отказ и закрыто", color: "#c2487a", statuses: ["REJECTED", "CANCELLED", "EXPIRED"] },
];

const DAY = 86_400_000;

// isStale: a manual case received more than a day ago; the tile and the table filter share it.
export function isStale(c: CaseListItem, now = Date.now()): boolean {
  return c.status === "MANUAL_REVIEW" && now - new Date(c.created_at).getTime() > DAY;
}

type Props = {
  items: CaseListItem[];
  partial: boolean;
  me: string | undefined;
  onFilter: (filter: { status?: BankStatus; assignee?: string; stale?: boolean }) => void;
};

// QueueSummary answers «what needs me now» before the analyst reads the table.
export function QueueSummary({ items, partial, me, onFilter }: Props) {
  const manual = items.filter((c) => c.status === "MANUAL_REVIEW");
  const free = manual.filter((c) => !c.assignee_id).length;
  const mine = manual.filter((c) => c.assignee_id === me).length;
  const stale = manual.filter((c) => isStale(c)).length;
  const counts = groups.map((g) => ({ ...g, n: items.filter((c) => g.statuses.includes(c.status)).length }));
  const total = items.length || 1;

  return (
    <section className="summary-board" aria-label="Сводка очереди">
      <div className="tiles">
        <button type="button" className="tile tile-main" onClick={() => onFilter({ status: "MANUAL_REVIEW" })}>
          <span className="tile-label">Ручной разбор</span>
          <span className="tile-value">{manual.length}</span>
          <span className="tile-note">
            свободных {free}, у вас {mine}
          </span>
        </button>
        <button type="button" className={`tile${stale ? " tile-alert" : ""}`} onClick={() => onFilter({ status: "MANUAL_REVIEW", stale: true })}>
          <span className="tile-label">Ждут больше суток</span>
          <span className="tile-value">{stale}</span>
          <span className="tile-note">{stale ? "начните с самых старых" : "таких дел нет"}</span>
        </button>
        <button type="button" className="tile" onClick={() => onFilter({ status: "MANUAL_REVIEW", assignee: "unassigned" })}>
          <span className="tile-label">Свободные</span>
          <span className="tile-value">{free}</span>
          <span className="tile-note">можно взять в работу</span>
        </button>
      </div>

      <figure className="dist">
        <figcaption>
          Все дела по этапам: {items.length}
          {partial && <span className="muted"> (последние 100)</span>}
        </figcaption>
        <div className="dist-bar" role="img" aria-label={counts.map((g) => `${g.label}: ${g.n}`).join(", ")}>
          {counts
            .filter((g) => g.n > 0)
            .map((g) => (
              <span key={g.key} className="dist-seg" style={{ flexGrow: g.n / total, background: g.color }} title={`${g.label}: ${g.n}`}>
                {g.n / total >= 0.12 && <b>{g.n}</b>}
              </span>
            ))}
        </div>
        <ul className="dist-legend">
          {counts.map((g) => (
            <li key={g.key}>
              <span className="key" style={{ background: g.color }} aria-hidden />
              {g.label}
              <strong>{g.n}</strong>
            </li>
          ))}
        </ul>
      </figure>
    </section>
  );
}
