import type { BankCase, BankStatus, ClientStatus } from "../api/types.ts";

// Match the public status events, not the bank's internal processing stages.
const clientStatuses: Record<BankStatus, ClientStatus> = {
  SUBMITTED: "UNDER_REVIEW",
  VALIDATION: "UNDER_REVIEW",
  ENRICHMENT: "UNDER_REVIEW",
  SCORING: "UNDER_REVIEW",
  DECISION: "UNDER_REVIEW",
  MANUAL_REVIEW: "UNDER_REVIEW",
  NEEDS_INFO: "NEEDS_INFO",
  OFFER: "OFFER_AVAILABLE",
  COUNTER_OFFER: "OFFER_AVAILABLE",
  OFFER_ACCEPTED: "FORMALIZING",
  SIGNING: "FORMALIZING",
  DISBURSED: "DISBURSED",
  REJECTED: "REJECTED",
  CANCELLED: "CLOSED",
  EXPIRED: "CLOSED",
};

type Communication = { status: ClientStatus } & (
  | { kind: "status" }
  | { kind: "request"; request: NonNullable<BankCase["info_request"]> }
  | { kind: "offer"; offer: NonNullable<BankCase["offer"]> }
  | { kind: "message"; message: string }
);

// The client renders request.message, offer.explanation, or a rejection reason.
// Other public_reason values are stored but are not separate client messages.
export function clientCommunication(kase: Pick<BankCase, "status" | "info_request" | "offer" | "public_reason">): Communication {
  const status = clientStatuses[kase.status];
  if (kase.status === "NEEDS_INFO" && kase.info_request) {
    return { kind: "request", status, request: kase.info_request };
  }
  if ((kase.status === "OFFER" || kase.status === "COUNTER_OFFER") && kase.offer) {
    return { kind: "offer", status, offer: kase.offer };
  }
  if (kase.status === "REJECTED" && kase.public_reason?.trim()) {
    return { kind: "message", status, message: kase.public_reason };
  }
  return { kind: "status", status };
}
