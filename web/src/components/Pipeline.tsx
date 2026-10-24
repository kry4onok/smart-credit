import type { BankStatus, ClientStatus } from "../api/types";
import { bankStatusLabel, clientStatusLabel, type Tone } from "../lib/labels";

type Props = {
  stations: string[];
  current: number;
  currentLabel?: string;
  tone: Tone;
  // The bank is working on the application: the current station pulses.
  waiting?: boolean;
  // Terminal outcome before the end of the line: later stations are unreachable.
  stopped?: boolean;
};

const stateText = { done: ", пройден", current: ", текущий этап", todo: ", впереди", skipped: ", не будет пройден" };

// Pipeline draws the credit conveyor as a line of stations with the application's position on it.
export function Pipeline({ stations, current, currentLabel, tone, waiting, stopped }: Props) {
  return (
    <ol className={`track track-${tone}`} aria-label="Этапы заявки">
      {stations.map((label, i) => {
        const state = i < current ? "done" : i === current ? "current" : stopped ? "skipped" : "todo";
        return (
          <li key={label} className={`station station-${state}${state === "current" && waiting ? " station-waiting" : ""}`} aria-current={state === "current" ? "step" : undefined}>
            <span className="station-dot" aria-hidden />
            <span className="station-label">{state === "current" && currentLabel ? currentLabel : label}</span>
            <span className="visually-hidden">{stateText[state]}</span>
          </li>
        );
      })}
    </ol>
  );
}

const clientStations = ["Черновик", "Отправлена", "Рассмотрение", "Решение банка", "Оформление", "Деньги на счёте"];

const clientIndex: Record<ClientStatus, number> = {
  DRAFT: 0,
  SUBMITTED: 1,
  UNDER_REVIEW: 2,
  NEEDS_INFO: 2,
  OFFER_AVAILABLE: 3,
  REJECTED: 3,
  CLOSED: 3,
  FORMALIZING: 4,
  DISBURSED: 5,
};

const clientTone: Record<ClientStatus, Tone> = {
  DRAFT: "neutral",
  SUBMITTED: "progress",
  UNDER_REVIEW: "progress",
  NEEDS_INFO: "attention",
  OFFER_AVAILABLE: "success",
  FORMALIZING: "success",
  DISBURSED: "success",
  REJECTED: "danger",
  CLOSED: "muted",
};

export function ClientPipeline({ status, waiting }: { status: ClientStatus; waiting: boolean }) {
  return (
    <Pipeline
      stations={clientStations}
      current={clientIndex[status]}
      currentLabel={clientStatusLabel[status]}
      tone={clientTone[status]}
      waiting={waiting}
      stopped={status === "REJECTED" || status === "CLOSED"}
    />
  );
}

const bankStations = ["Получена", "Полнота", "Источники", "Скоринг", "Решение", "Оформление", "Выдача"];

const bankIndex: Record<BankStatus, number> = {
  SUBMITTED: 0,
  VALIDATION: 1,
  NEEDS_INFO: 1,
  ENRICHMENT: 2,
  SCORING: 3,
  DECISION: 4,
  MANUAL_REVIEW: 4,
  OFFER: 4,
  COUNTER_OFFER: 4,
  REJECTED: 4,
  CANCELLED: 4,
  EXPIRED: 4,
  OFFER_ACCEPTED: 5,
  SIGNING: 5,
  DISBURSED: 6,
};

const bankTone: Record<BankStatus, Tone> = {
  SUBMITTED: "progress",
  VALIDATION: "progress",
  NEEDS_INFO: "attention",
  ENRICHMENT: "progress",
  SCORING: "progress",
  DECISION: "progress",
  MANUAL_REVIEW: "attention",
  OFFER: "success",
  COUNTER_OFFER: "success",
  OFFER_ACCEPTED: "success",
  SIGNING: "success",
  DISBURSED: "success",
  REJECTED: "danger",
  CANCELLED: "muted",
  EXPIRED: "muted",
};

const automatic: BankStatus[] = ["SUBMITTED", "VALIDATION", "ENRICHMENT", "SCORING", "DECISION", "SIGNING"];

export function BankPipeline({ status }: { status: BankStatus }) {
  return (
    <Pipeline
      stations={bankStations}
      current={bankIndex[status]}
      currentLabel={bankStatusLabel[status]}
      tone={bankTone[status]}
      waiting={automatic.includes(status)}
      stopped={status === "REJECTED" || status === "CANCELLED" || status === "EXPIRED"}
    />
  );
}

// MiniTrack is the compact line for lists; the status text next to it carries the meaning.
export function MiniTrack({ status }: { status: ClientStatus }) {
  const current = clientIndex[status];
  const stopped = status === "REJECTED" || status === "CLOSED";
  return (
    <span className={`mini-track track-${clientTone[status]}`} aria-hidden="true">
      {clientStations.map((label, i) => (
        <span key={label} className={i < current ? "done" : i === current ? "current" : stopped ? "skipped" : ""} />
      ))}
    </span>
  );
}
