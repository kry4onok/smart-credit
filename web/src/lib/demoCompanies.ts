import companies from "../../../pkg/demodata/companies.json";
import type { CompanyProfile } from "../api/types";

// The same public, fictional catalogue is embedded in auth and bureau-mocks.
export const demoCompanies = companies;
export const demoCompany = (inn: string | undefined) => companies.find((c) => c.inn === inn);

export function demoProfile(c: (typeof companies)[number], now = new Date()): CompanyProfile {
  const from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 12, 1)).toISOString().slice(0, 10);
  const to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0)).toISOString().slice(0, 10);
  return {
    profile_schema_version: 2,
    inn: c.inn, name: c.name, legal_form: c.inn.length === 12 ? "IP" : "LLC",
    okved: c.okved, registered_at: c.registered_at,
    tax_periods: [{ from: c.registered_at, regimes: [c.tax_regime as "PSN" | "USN_INCOME"], vat_status: "EXEMPT" }],
    financial_period: { from, to }, financial_basis: "MANAGEMENT",
    obligations_as_of: now.toISOString().slice(0, 10),
    revenue_12m: { amount_kopecks: c.monthly_receipts_kopecks * 12, currency: "RUB" },
    // Accounting profit is deliberately left unknown; cash flow is provided by the demo ledger.
    obligations: { amount_kopecks: c.debt_kopecks, currency: "RUB" },
  };
}
