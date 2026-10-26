import type { BankCase } from "../../api/types";
import { CompareBars, offerRows } from "../../components/Compare";
import { clientCommunication } from "../../lib/clientCommunication";
import { date, dateTime, money, months, rate } from "../../lib/format";
import { clientStatusLabel, documentLabel, infoFieldLabel } from "../../lib/labels";

export function ClientCommunication({ kase }: { kase: BankCase }) {
  const content = clientCommunication(kase);
  const status = <p className="field-hint">Статус у клиента: <strong>{clientStatusLabel[content.status]}</strong></p>;

  if (content.kind === "status") return status;

  return (
    <section className="panel">
      <h2>Сообщение клиенту</h2>
      {status}
      {content.kind === "request" && (
        <div className="sub">
          <h3>Запрос данных</h3>
          <p>{content.request.message}</p>
          <p className="field-hint">Запрос от {dateTime(content.request.created_at)}</p>
          {(content.request.requirements?.length ?? 0) > 0 ? (
            <ul>
              {content.request.requirements!.map((requirement) => (
                <li key={requirement.id}>
                  {requirement.label}
                  {requirement.period && <> · {date(requirement.period.from)} — {date(requirement.period.to)}</>}
                </li>
              ))}
            </ul>
          ) : (
            <ul>
              {content.request.required_fields.map((field) => <li key={field}>{infoFieldLabel[field]}</li>)}
              {content.request.required_document_types.map((document) => <li key={document}>{documentLabel[document]}</li>)}
            </ul>
          )}
        </div>
      )}
      {content.kind === "offer" && (
        <div className="sub">
          <h3>{content.offer.kind === "COUNTER_OFFER" ? "Другие условия кредита" : "Предложение банка"}</h3>
          <p>{content.offer.product_name}: {money(content.offer.amount)} на {months(content.offer.term_months)} под {rate(content.offer.rate_bps)}.</p>
          <p className="field-hint">Предложение действует до {dateTime(content.offer.expires_at)}</p>
          {(content.offer.kind === "COUNTER_OFFER" || kase.application.amount.amount_kopecks !== content.offer.amount.amount_kopecks || kase.application.term_months !== content.offer.term_months) && (
            <CompareBars rows={offerRows(kase.application, content.offer)} />
          )}
          {content.offer.explanation && <p>{content.offer.explanation}</p>}
        </div>
      )}
      {content.kind === "message" && <p>{content.message}</p>}
    </section>
  );
}
