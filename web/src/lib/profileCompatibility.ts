import type { CompanyProfile } from "../api/types";

export type FinancialCompany = CompanyProfile & Required<Pick<CompanyProfile, "revenue_12m" | "profit_12m" | "obligations">>;

export function hasLegacyFinancials(profile: CompanyProfile): profile is FinancialCompany {
  return profile.revenue_12m != null && profile.profit_12m != null && profile.obligations != null;
}

export function hasProfileV2Fields(profile: CompanyProfile): boolean {
  return profile.profile_schema_version !== undefined || profile.legal_form !== undefined ||
    profile.tax_periods !== undefined || profile.financial_period !== undefined ||
    profile.financial_basis !== undefined || profile.obligations_as_of !== undefined;
}
