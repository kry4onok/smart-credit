import { useEffect, useState } from "react";
import { clientApi } from "../../api/http";
import { call } from "../../api/problem";
import type { ApplicationFormPatch, ClientApplication, Offer, Product, PurposeTag } from "../../api/types";
import { months, moneyShort, rub } from "../../lib/format";

let cache: Promise<Product[]> | null = null;

// The catalog changes rarely; one request per page load is enough to name products in lists.
export function loadProducts(): Promise<Product[]> {
  cache ??= call(clientApi.GET("/client/products")).then((r) => r.items).catch((e) => {
    cache = null;
    throw e;
  });
  return cache;
}

export function useProductNames(): Record<string, Product> {
  const [byId, setById] = useState<Record<string, Product>>({});
  useEffect(() => {
    loadProducts()
      .then((items) => setById(Object.fromEntries(items.map((p) => [p.id, p]))))
      .catch(() => undefined);
  }, []);
  return byId;
}

// What the client already told the catalog: carried into the new draft so it is not typed twice.
export type Prefill = { amount?: number; term?: number; purpose?: PurposeTag };

export function prefillFromParams(params: URLSearchParams): Prefill {
  const amount = Number(params.get("amount"));
  const term = Number(params.get("term"));
  const purpose = params.get("purpose") as PurposeTag | null;
  return {
    amount: Number.isSafeInteger(amount) && amount > 0 ? amount * 100 : undefined,
    term: Number.isInteger(term) && term > 0 ? term : undefined,
    purpose: purpose ?? undefined,
  };
}

export function prefillToParams(p: Prefill): Record<string, string> {
  const out: Record<string, string> = {};
  if (p.amount) out.amount = String(Math.floor(p.amount / 100));
  if (p.term) out.term = String(p.term);
  if (p.purpose) out.purpose = p.purpose;
  return out;
}

export type DraftValues = { amount?: number | null; term?: number; purpose?: PurposeTag };

const draftKey = (id: string) => `smart-credit:draft:${id}`;

// client-service returns `form` only once amount, term and purpose are all set. Until then the
// browser keeps what was typed for this draft, so a reload or a return later does not blank the fields.
export function rememberDraft(id: string, values: DraftValues) {
  try {
    localStorage.setItem(draftKey(id), JSON.stringify(values));
  } catch {
    // Storage can be unavailable (private window); the fields then start empty after a reload.
  }
}

export function recallDraft(id: string): DraftValues {
  try {
    return JSON.parse(localStorage.getItem(draftKey(id)) ?? "{}") as DraftValues;
  } catch {
    return {};
  }
}

const offerKey = (id: string) => `smart-credit:accepted-offer:${id}`;

// After acceptance client-service stops returning the offer, yet /sign must name its offer_id.
// The accepted offer is kept here so the formalizing step can show the terms and sign them.
export function rememberOffer(applicationId: string, offer: Offer) {
  try {
    localStorage.setItem(offerKey(applicationId), JSON.stringify(offer));
  } catch {
    // Without storage the signing step explains that the terms are unavailable in this browser.
  }
}

export function recallOffer(applicationId: string): Offer | undefined {
  try {
    const raw = localStorage.getItem(offerKey(applicationId));
    return raw ? (JSON.parse(raw) as Offer) : undefined;
  } catch {
    return undefined;
  }
}

// Creates a draft and fills in the known terms. If the autosave part fails, the draft still exists.
export async function createDraft(product: Product, prefill: Prefill, key: string): Promise<ClientApplication> {
  const draft = await call(
    clientApi.POST("/client/applications", { params: { header: { "Idempotency-Key": key } }, body: { product_id: product.id } }),
  );
  const patch: ApplicationFormPatch = {};
  if (prefill.amount) patch.amount = rub(prefill.amount);
  if (prefill.term) patch.term_months = prefill.term;
  const purpose = prefill.purpose && product.purpose_tags.includes(prefill.purpose) ? prefill.purpose : product.purpose_tags.length === 1 ? product.purpose_tags[0] : undefined;
  if (purpose) patch.purpose = purpose;
  if (!Object.keys(patch).length) return draft;
  rememberDraft(draft.id, { amount: prefill.amount, term: prefill.term, purpose });
  try {
    return await call(
      clientApi.PATCH("/client/applications/{application_id}", {
        params: { path: { application_id: draft.id }, header: { "If-Match": `"${draft.version}"` } },
        body: patch,
        headers: { "Content-Type": "application/merge-patch+json" },
      }),
    );
  } catch {
    return draft;
  }
}

// Why a product does not fit the requested amount and term; null when it fits.
export function misfit(p: Product, amount: number | undefined, term: number | undefined): string | null {
  if (amount !== undefined && amount < p.min_amount.amount_kopecks) return `Сумма от ${moneyShort(p.min_amount)}`;
  if (amount !== undefined && amount > p.max_amount.amount_kopecks) return `Сумма до ${moneyShort(p.max_amount)}`;
  if (term !== undefined && term < p.min_term_months) return `Срок от ${months(p.min_term_months)}`;
  if (term !== undefined && term > p.max_term_months) return `Срок до ${months(p.max_term_months)}`;
  return null;
}
