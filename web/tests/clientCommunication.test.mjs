import assert from "node:assert/strict";
import test from "node:test";
import { clientCommunication } from "../src/lib/clientCommunication.ts";

const request = { id: "request", message: "Передайте выписку за прошлый год", required_fields: [], required_document_types: ["BANK_STATEMENT"], created_at: "2026-10-26T12:00:00Z" };
const offer = { id: "offer", explanation: "Предлагаем меньшую сумму на больший срок" };

test("internal assessment stages show only the public review status, never the generic reason or old content", () => {
  for (const status of ["SUBMITTED", "VALIDATION", "ENRICHMENT", "SCORING", "DECISION", "MANUAL_REVIEW"]) {
    assert.deepEqual(clientCommunication({ status, public_reason: "Расчёт подготовлен. Условия подтверждает сотрудник банка.", info_request: request, offer }), { kind: "status", status: "UNDER_REVIEW" });
  }
});

test("an information request uses its actual message and requirements, not a second public reason", () => {
  const withRequirements = { ...request, required_document_types: [], requirements: [{ id: "patent", label: "Патент", kind: "DOCUMENT" }] };
  assert.deepEqual(clientCommunication({ status: "NEEDS_INFO", info_request: withRequirements, public_reason: "Общий шаблон", offer }), { kind: "request", status: "NEEDS_INFO", request: withRequirements });
});

test("offer and counteroffer use only the offer explanation seen in the client cabinet", () => {
  for (const status of ["OFFER", "COUNTER_OFFER"]) {
    assert.deepEqual(clientCommunication({ status, offer, info_request: request, public_reason: "Старое объяснение" }), { kind: "offer", status: "OFFER_AVAILABLE", offer });
  }
});

test("rejection preserves the published wording and excludes old requests and offers", () => {
  assert.deepEqual(clientCommunication({ status: "REJECTED", public_reason: "Продукт не подходит для указанной цели.", offer, info_request: request }), { kind: "message", status: "REJECTED", message: "Продукт не подходит для указанной цели." });
});

test("accepted, signed, disbursed and closed states do not republish old decision messages", () => {
  for (const [status, expected] of [["OFFER_ACCEPTED", "FORMALIZING"], ["SIGNING", "FORMALIZING"], ["DISBURSED", "DISBURSED"], ["CANCELLED", "CLOSED"], ["EXPIRED", "CLOSED"]]) {
    assert.deepEqual(clientCommunication({ status, public_reason: "Старое объяснение", offer, info_request: request }), { kind: "status", status: expected });
  }
});

test("absent messages are not invented from public_reason or another stage's payload", () => {
  for (const [status, expected] of [["NEEDS_INFO", "NEEDS_INFO"], ["OFFER", "OFFER_AVAILABLE"], ["COUNTER_OFFER", "OFFER_AVAILABLE"]]) {
    assert.deepEqual(clientCommunication({ status, public_reason: "Расчёт подготовлен" }), { kind: "status", status: expected });
  }
  assert.deepEqual(clientCommunication({ status: "REJECTED", public_reason: "  " }), { kind: "status", status: "REJECTED" });
});
