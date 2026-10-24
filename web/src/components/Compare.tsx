import type { Money } from "../api/types";
import { money, months } from "../lib/format";

type Row = { label: string; requested: number; offered: number; format: (n: number) => string };

// CompareBars puts what the client asked for next to what the bank offers, one pair of bars per term.
export function CompareBars({ rows }: { rows: Row[] }) {
  return (
    <figure className="compare">
      <figcaption className="compare-legend">
        <span className="key key-req" aria-hidden />
        Запрошено
        <span className="key key-off" aria-hidden />
        Предложено
      </figcaption>
      {rows.map((r) => {
        const max = Math.max(r.requested, r.offered, 1);
        const delta = r.requested ? Math.round(((r.offered - r.requested) / r.requested) * 100) : 0;
        return (
          <div key={r.label} className="compare-row">
            <span className="compare-label">{r.label}</span>
            <div className="compare-bars">
              <span className="visually-hidden">
                запрошено {r.format(r.requested)}, предложено {r.format(r.offered)}
              </span>
              <span className="cbar" aria-hidden title={`Запрошено: ${r.format(r.requested)}`}>
                <span className="cbar-fill cbar-req" style={{ width: `${(r.requested / max) * 100}%` }} />
                <span className="cbar-value">{r.format(r.requested)}</span>
              </span>
              <span className="cbar" aria-hidden title={`Предложено: ${r.format(r.offered)}`}>
                <span className="cbar-fill cbar-off" style={{ width: `${(r.offered / max) * 100}%` }} />
                <span className="cbar-value">{r.format(r.offered)}</span>
              </span>
            </div>
            <span className={`compare-delta${delta < 0 ? " down" : delta > 0 ? " up" : ""}`}>
              {delta === 0 ? "как в заявке" : `${delta > 0 ? "+" : "−"}${Math.abs(delta)}%`}
            </span>
          </div>
        );
      })}
    </figure>
  );
}

export function offerRows(requested: { amount: Money; term_months: number }, offered: { amount: Money; term_months: number }): Row[] {
  return [
    { label: "Сумма", requested: requested.amount.amount_kopecks, offered: offered.amount.amount_kopecks, format: (n) => money({ amount_kopecks: n, currency: "RUB" }) },
    { label: "Срок", requested: requested.term_months, offered: offered.term_months, format: months },
  ];
}
