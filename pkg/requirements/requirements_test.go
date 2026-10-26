package requirements

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fixture struct {
	Cases []struct {
		ScenarioID    string `json:"scenario_id"`
		ReferenceDate string `json:"reference_date"`
		Profile       struct {
			Inn                  string
			Name                 string
			Okved                string
			RegisteredAt         string `json:"registered_at"`
			LegalForm            string `json:"legal_form"`
			FinancialBasis       string `json:"financial_basis"`
			ObligationsAsOf      string `json:"obligations_as_of"`
			ProfileSchemaVersion int32  `json:"profile_schema_version"`
			TaxPeriods           []struct {
				From      string
				To        *string
				Regimes   []string
				VatStatus string `json:"vat_status"`
			} `json:"tax_periods"`
			Revenue         *moneyJSON  `json:"revenue_12m"`
			Profit          *moneyJSON  `json:"profit_12m"`
			Obligations     *moneyJSON  `json:"obligations"`
			FinancialPeriod *periodJSON `json:"financial_period"`
		} `json:"profile"`
		Product struct {
			ID, Code, Policy          string
			Min                       *moneyJSON `json:"min_amount"`
			Max                       *moneyJSON `json:"max_amount"`
			MinTerm                   int32      `json:"min_term_months"`
			MaxTerm                   int32      `json:"max_term_months"`
			Purposes                  []string   `json:"purpose_tags"`
			RequirementsPolicyVersion string     `json:"requirements_policy_version"`
		} `json:"product"`
		Form struct {
			Amount  *moneyJSON
			Term    int32 `json:"term_months"`
			Purpose string
		} `json:"form"`
		Expected struct {
			Route           *string
			CanSubmit       bool     `json:"can_submit"`
			ValidationError *string  `json:"validation_error"`
			AssessmentCodes []string `json:"assessment_codes"`
		} `json:"expected"`
	} `json:"cases"`
}
type moneyJSON struct {
	Amount   int64 `json:"amount_kopecks"`
	Currency string
}
type periodJSON struct{ From, To string }

func money(m *moneyJSON) *commonpb.Money {
	if m == nil {
		return nil
	}
	return &commonpb.Money{AmountKopecks: m.Amount, Currency: m.Currency}
}
func TestProfileRequirementsMatrix(t *testing.T) {
	b, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var data fixture
	if err = json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 16 {
		t.Fatalf("expected 16 cases, got %d", len(data.Cases))
	}
	for _, tc := range data.Cases {
		t.Run(tc.ScenarioID, func(t *testing.T) {
			p := tc.Profile
			c := &commonpb.CompanySnapshot{Inn: p.Inn, Name: p.Name, Okved: p.Okved, RegisteredAt: p.RegisteredAt, ProfileSchemaVersion: &p.ProfileSchemaVersion, LegalForm: commonpb.LegalForm(commonpb.LegalForm_value["LEGAL_FORM_"+p.LegalForm]), Revenue_12M: money(p.Revenue), Profit_12M: money(p.Profit), Obligations: money(p.Obligations), FinancialBasis: commonpb.FinancialBasis(commonpb.FinancialBasis_value["FINANCIAL_BASIS_"+p.FinancialBasis]), ObligationsAsOf: p.ObligationsAsOf}
			if p.FinancialPeriod != nil {
				c.FinancialPeriod = &commonpb.DatePeriod{From: p.FinancialPeriod.From, To: p.FinancialPeriod.To}
			}
			for _, tp := range p.TaxPeriods {
				q := &commonpb.TaxPeriod{From: tp.From, To: tp.To, VatStatus: commonpb.VatStatus(commonpb.VatStatus_value["VAT_STATUS_"+tp.VatStatus])}
				for _, r := range tp.Regimes {
					q.Regimes = append(q.Regimes, commonpb.TaxRegime(commonpb.TaxRegime_value["TAX_REGIME_"+r]))
				}
				c.TaxPeriods = append(c.TaxPeriods, q)
			}
			pr := tc.Product
			product := &commonpb.Product{RequirementsPolicyVersion: pr.RequirementsPolicyVersion, MinAmount: money(pr.Min), MaxAmount: money(pr.Max), MinTermMonths: pr.MinTerm, MaxTermMonths: pr.MaxTerm}
			for _, p := range pr.Purposes {
				product.PurposeTags = append(product.PurposeTags, commonpb.PurposeTag(commonpb.PurposeTag_value["PURPOSE_TAG_"+p]))
			}
			form := &commonpb.ApplicationForm{Amount: money(tc.Form.Amount), TermMonths: tc.Form.Term, Purpose: commonpb.PurposeTag(commonpb.PurposeTag_value["PURPOSE_TAG_"+tc.Form.Purpose])}
			date, _ := day(tc.ReferenceDate)
			plan, err := Build(Input{Company: c, Product: product, Form: form, ReferenceDate: date})
			if plan == nil {
				t.Fatalf("nil plan: %v", err)
			}
			if tc.Expected.ValidationError == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.Expected.ValidationError != nil {
				var re *RuleError
				if !errors.As(err, &re) || re.Code != *tc.Expected.ValidationError {
					t.Fatalf("error %v, want %s", err, *tc.Expected.ValidationError)
				}
			}
			if plan.CanSubmit != tc.Expected.CanSubmit {
				t.Errorf("can_submit=%v", plan.CanSubmit)
			}
			if tc.Expected.Route != nil && strings.TrimPrefix(plan.GetRoute().String(), "REQUIREMENTS_ROUTE_") != *tc.Expected.Route {
				t.Errorf("route=%v, want %s", plan.Route, *tc.Expected.Route)
			}
			got := []string{}
			for _, r := range plan.Items {
				if r.GetRequiredAt() == commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT {
					got = append(got, r.GetCode())
				}
			}
			sort.Strings(got)
			want := append([]string{}, tc.Expected.AssessmentCodes...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("assessment=%v, want %v", got, want)
			}
		})
	}
}

func TestDocumentCoverageAndIdentity(t *testing.T) {
	r := doc("CASH_FLOW", commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT, &commonpb.DatePeriod{From: "2025-10-01", To: "2026-09-30"}, 0, commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT)
	if r.Id != "cash_flow:2025-10-01:2026-09-30:all" {
		t.Fatal(r.Id)
	}
	docs := []*commonpb.DocumentMetadata{
		{DocumentId: "wrong-period", RequirementId: r.Id, DocumentType: commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT, CoveragePeriod: &commonpb.DatePeriod{From: "2026-01-01", To: "2026-09-30"}},
		{DocumentId: "wrong-id", RequirementId: "other", DocumentType: commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT, CoveragePeriod: r.Period},
		{DocumentId: "old", RequirementId: r.Id, DocumentType: commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT, CoveragePeriod: r.Period, UploadedAt: timestamppb.New(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))},
		{DocumentId: "new", RequirementId: r.Id, DocumentType: commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT, CoveragePeriod: r.Period, UploadedAt: timestamppb.New(time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))},
	}
	if got := MatchDocuments(r, docs); !reflect.DeepEqual(got, []string{"new", "old"}) {
		t.Fatalf("matches=%v", got)
	}
	plan := &commonpb.RequirementsPlan{Items: []*commonpb.Requirement{r}}
	if _, err := SelectRequest(plan, []string{r.Id, r.Id}); err == nil {
		t.Fatal("duplicate accepted")
	}
	if _, err := SelectRequest(plan, []string{"unknown"}); err == nil {
		t.Fatal("unknown accepted")
	}
	selected, err := SelectRequest(plan, []string{r.Id})
	if err != nil || len(selected) != 1 {
		t.Fatalf("selection %v %v", selected, err)
	}
	current := &commonpb.RequirementsPlan{Items: []*commonpb.Requirement{r}}
	if len(ValidateResponse(selected, current, nil)) != 1 {
		t.Fatal("missing document accepted")
	}
	r.Status = commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED
	if len(ValidateResponse(selected, current, nil)) != 0 {
		t.Fatal("provided document rejected")
	}
}

func TestProfilePresenceAndTaxPeriods(t *testing.T) {
	m := &commonpb.Money{AmountKopecks: 0, Currency: "RUB"}
	if m == nil || m.GetAmountKopecks() != 0 {
		t.Fatal("zero lost")
	}
	f := field("company.profit_12m", m != nil)
	if f.Status != commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED {
		t.Fatal("zero treated as absent")
	}
	if field("company.profit_12m", false).Status != commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING {
		t.Fatal("nil treated as zero")
	}
	if taxCombination(commonpb.LegalForm_LEGAL_FORM_LLC, []commonpb.TaxRegime{commonpb.TaxRegime_TAX_REGIME_PSN}) {
		t.Fatal("LLC PSN accepted")
	}
	if !taxCombination(commonpb.LegalForm_LEGAL_FORM_IP, []commonpb.TaxRegime{commonpb.TaxRegime_TAX_REGIME_USN_INCOME, commonpb.TaxRegime_TAX_REGIME_PSN}) {
		t.Fatal("IP USN+PSN rejected")
	}
	requested := field("company.profit_12m", true)
	now := &commonpb.RequirementsPlan{Items: []*commonpb.Requirement{requested}}
	if len(ValidateResponse([]*commonpb.Requirement{requested}, now, nil)) == 0 {
		t.Fatal("old PROVIDED closed repeated request")
	}
	if issues := ValidateResponse([]*commonpb.Requirement{requested}, now, []string{requested.Id}); len(issues) != 0 {
		t.Fatal(issues)
	}
}

func TestAccountingCutoffAndStableIDs(t *testing.T) {
	version := int32(2)
	c := &commonpb.CompanySnapshot{
		Inn: "9900000017", Name: "ООО Демо-профиль", Okved: "47.11", RegisteredAt: "2020-01-01",
		ProfileSchemaVersion: &version, LegalForm: commonpb.LegalForm_LEGAL_FORM_LLC,
		TaxPeriods:  []*commonpb.TaxPeriod{{From: "2020-01-01", Regimes: []commonpb.TaxRegime{commonpb.TaxRegime_TAX_REGIME_USN_INCOME}, VatStatus: commonpb.VatStatus_VAT_STATUS_UNKNOWN}},
		Revenue_12M: &commonpb.Money{AmountKopecks: 0, Currency: "RUB"}, Profit_12M: &commonpb.Money{AmountKopecks: -1, Currency: "RUB"}, Obligations: &commonpb.Money{AmountKopecks: 0, Currency: "RUB"},
		FinancialBasis: commonpb.FinancialBasis_FINANCIAL_BASIS_MANAGEMENT, ObligationsAsOf: "2026-05-20",
	}
	product := &commonpb.Product{RequirementsPolicyVersion: policyVersion, MinAmount: &commonpb.Money{AmountKopecks: 1, Currency: "RUB"}, MaxAmount: &commonpb.Money{AmountKopecks: 100, Currency: "RUB"}, MinTermMonths: 1, MaxTermMonths: 12, PurposeTags: []commonpb.PurposeTag{commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL}}
	form := &commonpb.ApplicationForm{Amount: &commonpb.Money{AmountKopecks: 10, Currency: "RUB"}, TermMonths: 6, Purpose: commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL}
	for _, tc := range []struct {
		ref, from, to, reportID string
		reportType              commonpb.DocumentType
	}{
		{"2026-05-31", "2025-05-01", "2026-04-30", "business_result:2025-05-01:2026-04-30:all", commonpb.DocumentType_DOCUMENT_TYPE_MANAGEMENT_REPORT},
		{"2026-06-01", "2025-06-01", "2026-05-31", "business_result:2025-01-01:2025-12-31:all", commonpb.DocumentType_DOCUMENT_TYPE_ACCOUNTING_REPORT},
	} {
		c.FinancialPeriod = &commonpb.DatePeriod{From: tc.from, To: tc.to}
		c.ObligationsAsOf = tc.ref
		ref, _ := day(tc.ref)
		plan, err := Build(Input{Company: c, Product: product, Form: form, ReferenceDate: ref})
		if err != nil {
			t.Fatal(err)
		}
		if !plan.CanSubmit {
			t.Fatal("zero and negative known finances must permit submission")
		}
		found := false
		for _, r := range plan.Items {
			if r.Code == "BUSINESS_RESULT" {
				found = true
				if r.Id != tc.reportID || len(r.AcceptedDocumentTypes) != 1 || r.AcceptedDocumentTypes[0] != tc.reportType {
					t.Fatalf("%s: unexpected report %v", tc.ref, r)
				}
			}
		}
		if !found {
			t.Fatalf("%s: missing business result", tc.ref)
		}
	}
}

func TestSourceFirstPeriodDoesNotDependOnLegacyFinancialFields(t *testing.T) {
	c := &commonpb.CompanySnapshot{Inn: "9900000017", Name: "Demo", Okved: "47.11", RegisteredAt: "2020-01-01", LegalForm: commonpb.LegalForm_LEGAL_FORM_LLC, TaxPeriods: []*commonpb.TaxPeriod{{From: "2020-01-01", Regimes: []commonpb.TaxRegime{commonpb.TaxRegime_TAX_REGIME_USN_INCOME}, VatStatus: commonpb.VatStatus_VAT_STATUS_UNKNOWN}}}
	p := &commonpb.Product{RequirementsPolicyVersion: SourceFirstPolicyVersion, MinAmount: &commonpb.Money{AmountKopecks: 1, Currency: "RUB"}, MaxAmount: &commonpb.Money{AmountKopecks: 100, Currency: "RUB"}, MinTermMonths: 1, MaxTermMonths: 12, PurposeTags: []commonpb.PurposeTag{commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL}}
	f := &commonpb.ApplicationForm{Amount: &commonpb.Money{AmountKopecks: 10, Currency: "RUB"}, TermMonths: 6, Purpose: commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL}
	ref, _ := day("2026-10-26")
	plan, err := Build(Input{Company: c, Product: p, Form: f, ReferenceDate: ref})
	if err != nil || !plan.CanSubmit {
		t.Fatalf("%v %v", plan, err)
	}
	found := false
	for _, r := range plan.Items {
		if r.Code == "CASH_FLOW" {
			found = true
			if r.Period.From != "2025-10-01" || r.Period.To != "2026-09-30" {
				t.Fatal(r)
			}
		}
	}
	if !found {
		t.Fatal("source-first policy lost document coverage")
	}
	p.RequirementsPolicyVersion = policyVersion
	old, err := Build(Input{Company: c, Product: p, Form: f, ReferenceDate: ref})
	if err != nil || old.CanSubmit {
		t.Fatalf("legacy policy changed: %v %v", old, err)
	}
}

func TestOtherPurposeRequiresDescriptionAndProductSupport(t *testing.T) {
	c := &commonpb.CompanySnapshot{Inn: "9900000017", Name: "Demo", Okved: "47.11", RegisteredAt: "2020-01-01", LegalForm: commonpb.LegalForm_LEGAL_FORM_LLC}
	p := &commonpb.Product{RequirementsPolicyVersion: SourceFirstPolicyVersion, MinAmount: &commonpb.Money{AmountKopecks: 1, Currency: "RUB"}, MaxAmount: &commonpb.Money{AmountKopecks: 100, Currency: "RUB"}, MinTermMonths: 1, MaxTermMonths: 12, PurposeTags: []commonpb.PurposeTag{commonpb.PurposeTag_PURPOSE_TAG_OTHER}}
	f := &commonpb.ApplicationForm{Amount: &commonpb.Money{AmountKopecks: 10, Currency: "RUB"}, TermMonths: 6, Purpose: commonpb.PurposeTag_PURPOSE_TAG_OTHER}
	ref, _ := day("2026-10-26")
	for _, description := range []string{"", " \t", "Ремонт, техника и сырьё"} {
		f.PurposeDescription = description
		plan, err := Build(Input{Company: c, Product: p, Form: f, ReferenceDate: ref})
		want := strings.TrimSpace(description) != ""
		if err != nil || plan.CanSubmit != want {
			t.Fatalf("description=%q plan=%v err=%v", description, plan, err)
		}
		found := false
		for _, item := range plan.Items {
			if item.FieldPath == "form.purpose_description" {
				found = true
				if (item.Status == commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED) != want {
					t.Fatal(item)
				}
			}
		}
		if !found {
			t.Fatal("missing purpose description requirement")
		}
	}
	p.PurposeTags = []commonpb.PurposeTag{commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL}
	plan, err := Build(Input{Company: c, Product: p, Form: f, ReferenceDate: ref})
	if err != nil || plan.CanSubmit {
		t.Fatalf("unsupported purpose plan=%v err=%v", plan, err)
	}
}
