import type { BankAssessment, Candidate } from "../api/types";

// null identifies the legacy flow; a pending/expired v2 assessment stays blocked.
export type OfferBasis = { candidates: Candidate[]; stale: boolean; status?: string } | null;

export function offerBasis(data: BankAssessment | null, now = Date.now()): OfferBasis {
  if (!data) return null;
  const result = data.assessment;
  const fresh = data.freshness === "CURRENT" && !!result && Date.parse(result.valid_until) > now;
  return {
    stale: !fresh,
    status: result?.status,
    candidates: fresh ? (result.candidates ?? []).filter((c) => Date.parse(c.valid_until) > now) : [],
  };
}
