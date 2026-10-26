package assessment

import (
	"slices"
	"time"
)

func Checks(in Input) CheckResult {
	r := CheckResult{Conclusion: "CLEAR", Codes: []string{}, Sources: in.Sources.Checks, RegistrationDateIsActivityProxy: true}
	mark := func(conclusion, code string) {
		if !slices.Contains(r.Codes, code) {
			r.Codes = append(r.Codes, code)
		}
		if r.Conclusion != "BLOCK" {
			r.Conclusion = conclusion
		}
	}
	if in.Sources.SchemaVersion != 1 || in.Sources.INN != in.INN {
		mark("REVIEW", "SOURCE_IDENTITY_MISMATCH")
		return r
	}
	seen := map[string]bool{}
	trusted := map[string]bool{}
	required := map[string]bool{"REGISTRY": false, "CREDIT_HISTORY": false, "BANKRUPTCY": false, "ARBITRATION": false, "ENFORCEMENT": false}
	for _, s := range in.Sources.Checks {
		if s.ID == "" || seen[s.ID] || s.SubjectID == "" || !validProvenance(s.Provenance, in.AsOf) {
			mark("REVIEW", "INVALID_SOURCE")
			continue
		}
		seen[s.ID] = true
		if s.Role == "BORROWER" && s.SubjectID == in.INN {
			if _, ok := required[s.Kind]; ok {
				required[s.Kind] = true
			}
		}
		if s.Role != "BORROWER" && s.Role != "DIRECTOR" && s.Role != "OWNER" {
			mark("REVIEW", "INVALID_SUBJECT_ROLE")
			continue
		}
		if !s.ValidUntil.After(in.AsOf) || s.Status == "STALE" {
			mark("REVIEW", "SOURCE_STALE")
			continue
		}
		if s.Status == "AMBIGUOUS" || s.Match == "AMBIGUOUS" {
			mark("REVIEW", "SUBJECT_MATCH_AMBIGUOUS")
			continue
		}
		if s.Status == "NOT_FOUND" && slices.Contains([]string{"BANKRUPTCY", "ARBITRATION", "ENFORCEMENT"}, s.Kind) && s.Match == "EXACT" {
			continue
		}
		if s.Status != "SUCCESS" || s.Match != "EXACT" || s.Conclusion != "CLEAR" && s.Conclusion != "REVIEW" && s.Conclusion != "BLOCK" {
			mark("REVIEW", "SOURCE_UNAVAILABLE")
			continue
		}
		if s.Role == "BORROWER" && s.SubjectID != in.INN {
			mark("REVIEW", "SOURCE_IDENTITY_MISMATCH")
			continue
		}
		if s.Role == "BORROWER" {
			trusted[s.Kind] = true
		}
		if s.Conclusion == "BLOCK" && s.Role == "BORROWER" && s.SubjectID == in.INN {
			mark("BLOCK", s.Kind+"_BLOCK")
		} else if s.Conclusion != "CLEAR" {
			mark("REVIEW", "RELATED_OR_SOURCE_REVIEW")
		}
	}
	for _, kind := range []string{"REGISTRY", "CREDIT_HISTORY", "BANKRUPTCY", "ARBITRATION", "ENFORCEMENT"} {
		if !required[kind] {
			mark("REVIEW", kind+"_MISSING")
		}
	}
	if trusted["REGISTRY"] {
		registered, err := time.Parse(time.DateOnly, in.Sources.RegisteredAt)
		if err != nil || registered.After(in.AsOf) {
			mark("REVIEW", "REGISTRATION_UNKNOWN")
		} else if registered.AddDate(1, 0, 0).After(in.AsOf) {
			mark("BLOCK", "ACTIVITY_UNDER_12_MONTHS")
		}
		if in.Sources.CompanyStatus == "LIQUIDATED" || in.Sources.CompanyStatus == "LIQUIDATING" {
			mark("BLOCK", "COMPANY_LIQUIDATION")
		} else if in.Sources.CompanyStatus != "ACTIVE" {
			mark("REVIEW", "COMPANY_STATUS_UNKNOWN")
		}
	}
	c := in.Sources.Credit
	start := periodStart(in.AsOf)
	end := start.AddDate(1, 0, -1)
	validCredit := trusted["CREDIT_HISTORY"] && c.HistoricalMaxDPD != nil && c.CurrentDPD != nil && c.OverdueEpisodes != nil && c.TotalDebtKopecks != nil && c.PeriodFrom == start.Format(time.DateOnly) && c.PeriodTo == end.Format(time.DateOnly)
	if !validCredit {
		mark("REVIEW", "CREDIT_HISTORY_INCOMPLETE")
	} else {
		if *c.HistoricalMaxDPD < 0 || *c.CurrentDPD < 0 || *c.OverdueEpisodes < 0 || *c.TotalDebtKopecks < 0 || (*c.HistoricalMaxDPD == 0) != (*c.OverdueEpisodes == 0) {
			mark("REVIEW", "CREDIT_HISTORY_CONFLICT")
			return r
		}
		// Historical and current DPD are distinct. The current month is not silently
		// folded into the last 12 completed months.
		if *c.HistoricalMaxDPD >= 90 {
			mark("BLOCK", "HISTORICAL_DPD_90_12M")
		}
		if *c.CurrentDPD > 0 {
			mark("REVIEW", "CURRENT_OVERDUE")
		}
	}
	return r
}
