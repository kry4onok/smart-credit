package assessment

import (
	"errors"
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/creditcalc"
	"slices"
	"time"
)

func ValidateSubmission(s Submission, amount int64) error {
	if s.SchemaVersion != 1 {
		return errors.New("UNSUPPORTED_ASSESSMENT_SCHEMA")
	}
	p := s.Preferences
	if p.Priority != "" && !slices.Contains([]string{"KEEP_AMOUNT", "LOW_PAYMENT", "LOW_TOTAL_COST"}, p.Priority) || p.MinimumUsefulAmountKopecks != nil && (*p.MinimumUsefulAmountKopecks <= 0 || *p.MinimumUsefulAmountKopecks > amount) || p.MaxTermMonths != nil && (*p.MaxTermMonths < 1 || *p.MaxTermMonths > 120) || p.MaxPaymentKopecks != nil && (*p.MaxPaymentKopecks <= 0 || *p.MaxPaymentKopecks > creditcalc.MaxMoney) {
		return errors.New("INVALID_PREFERENCES")
	}
	if s.Financials != nil {
		if len(s.Financials.Months) > 12 || len(s.Financials.DebtPayments) > 1200 {
			return errors.New("INPUT_TOO_LARGE")
		}
		// Client transport cannot assert independently verified facts.
		if s.Financials.Provenance.Quality != "DECLARED" && s.Financials.Provenance.Quality != "PROVIDED" {
			return errors.New("CLIENT_CANNOT_VERIFY_DATA")
		}
		for _, m := range s.Financials.Months {
			if m.Provenance.Quality != "DECLARED" && m.Provenance.Quality != "PROVIDED" {
				return errors.New("CLIENT_CANNOT_VERIFY_DATA")
			}
		}
	}
	return nil
}

func Evaluate(in Input) Result {
	r := Result{SchemaVersion: 1, InputRevision: in.InputRevision, Generation: in.Generation, AsOf: in.AsOf, ValidUntil: validUntil(in), PolicyVersion: PolicyVersion, PolicyChecksum: hash(policyConfig), InputHash: hash(in), Status: "MANUAL_REVIEW", Candidates: []Candidate{}, Issues: []Issue{}, OriginalFailureCodes: []string{}, Changes: []Change{}}
	r.ID = hash(struct {
		Application          string
		Revision, Generation int64
		Hash                 string
	}{in.ApplicationID, in.InputRevision, in.Generation, r.InputHash})
	r.Checks = Checks(in)
	if in.AsOf.IsZero() || in.RequestedAmount <= 0 || in.RequestedAmount > maxComponent || in.RequestedTerm < 1 || in.RequestedTerm > 120 || len(in.Products) > 30 || in.InputRevision < 1 || in.Generation < 1 || in.LegalForm != "IP" && in.LegalForm != "LLC" {
		r.Issues = append(r.Issues, issue("INVALID_INPUT", "application", "Некорректные исходные данные"))
		return r
	}
	if err := ValidateSubmission(in.Submission, in.RequestedAmount); err != nil {
		r.Issues = append(r.Issues, issue(err.Error(), "preferences", "Проверьте предпочтения и происхождение данных"))
		r.Status = "NEEDS_INFO"
		return r
	}
	if r.Checks.Conclusion == "BLOCK" {
		r.Status = "REJECT"
		return r
	}
	if r.Checks.Conclusion != "CLEAR" {
		return r
	}
	f, issues := Normalize(in)
	r.Issues = issues
	if len(issues) > 0 {
		r.Status = "NEEDS_INFO"
		for _, i := range issues {
			if i.Code == "SOURCE_CONFLICT" || i.Code == "DEBT_CONFLICT" {
				r.Status = "MANUAL_REVIEW"
			}
		}
		return r
	}
	r.Financial = &f
	r.Risk = Score(f, in.Sources.Credit)
	if r.Risk.Status != "ASSESSED" {
		r.Status = "NEEDS_INFO"
		r.Issues = append(r.Issues, r.Risk.Issues...)
		return r
	}
	if r.Risk.Grade == "E" {
		r.Status = "REJECT"
		return r
	}
	if r.Risk.Grade != "A" && r.Risk.Grade != "B" {
		return r
	}
	var requested Product
	for _, p := range in.Products {
		if p.ID == in.RequestedProductID {
			requested = p
		}
	}
	_, err := checkTerms(in, f, r.Risk, in.RequestedProductID, creditcalc.Terms{AmountKopecks: in.RequestedAmount, TermMonths: in.RequestedTerm, RateBPS: rate(requested, r.Risk.Grade)})
	if err != nil {
		r.OriginalFailureCodes = append(r.OriginalFailureCodes, err.Error())
	}
	r.Candidates = Generate(in, f, r.Risk)
	if len(r.Candidates) > 0 {
		r.Status = "READY_FOR_CONFIRMATION"
	} else {
		r.Status = "NO_AFFORDABLE_OPTION"
	}
	return r
}

func Public(r Result) PublicResult {
	// Do not publish source check codes or internal risk factors in client JSON.
	return PublicResult{SchemaVersion: r.SchemaVersion, ID: r.ID, InputRevision: r.InputRevision, Generation: r.Generation, AsOf: r.AsOf, ValidUntil: r.ValidUntil, PolicyVersion: r.PolicyVersion, Status: r.Status, Financial: r.Financial, Candidates: r.Candidates, Issues: r.Issues, Changes: []Change{}}
}

func Diff(previous, current Result) []Change {
	out := []Change{}
	if previous.PolicyVersion != current.PolicyVersion {
		out = append(out, Change{"policy_version", previous.PolicyVersion, current.PolicyVersion, "VERSION"})
	}
	if previous.Risk.ModelVersion != current.Risk.ModelVersion {
		out = append(out, Change{"risk.model_version", previous.Risk.ModelVersion, current.Risk.ModelVersion, "VERSION"})
	}
	if previous.Risk.Score != nil && current.Risk.Score != nil && *previous.Risk.Score != *current.Risk.Score {
		out = append(out, Change{"risk.score", *previous.Risk.Score, *current.Risk.Score, "POINTS"})
	}
	if previous.Status != current.Status {
		out = append(out, Change{"status", previous.Status, current.Status, "STATUS"})
	}
	if previous.Financial != nil && current.Financial != nil && previous.Financial.MarginBPS != current.Financial.MarginBPS {
		out = append(out, Change{"financial.operating_margin_bps", previous.Financial.MarginBPS, current.Financial.MarginBPS, "BASIS_POINTS"})
	}
	return out
}

func IsCurrent(r Result, in Input, now time.Time) bool {
	return r.InputRevision == in.InputRevision && r.Generation == in.Generation && r.InputHash == hash(in) && r.PolicyVersion == PolicyVersion && r.ValidUntil.After(now)
}
