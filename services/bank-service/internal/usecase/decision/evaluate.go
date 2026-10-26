package decision

import (
	"encoding/json"
	"math/big"
	"strings"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
)

const (
	InfoMessage   = "Для продолжения рассмотрения дополните данные заявки."
	RejectMessage = "Банк не может предложить кредит по действующим правилам."
)

type Input struct {
	Snapshot      validation.Snapshot
	Product       Product
	Grade         string
	ScoreFound    bool
	CompanyStatus string
	CompanyOK     bool
	OverdueDays   int
	CreditOK      bool
	Now           time.Time
}

type Product struct {
	Found              bool
	Name               string
	Version            int64
	Purposes           []string
	Documents          []string
	MinAmount          int64
	MaxAmount          int64
	MinTerm            int
	MaxTerm            int
	MinAgeMonths       int
	MaxAmountRevenueBP int
	MaxOverdueDays     int
	GradeARateBP       int
	GradeBRateBP       int
	OfferTTLHours      int
	PolicyVersion      string
}

type Result struct {
	Status         model.CaseStatus
	Outcome        string
	Rules          []string
	PublicReason   string
	RequiredFields []string
	RequiredDocs   []string
	RateBP         int
	Plan           *commonpb.RequirementsPlan
	Requirements   []*commonpb.Requirement
}

func Evaluate(in Input) Result {
	manual := func(rule string) Result {
		return Result{Status: model.CaseStatusManualReview, Outcome: "MANUAL_REVIEW", Rules: []string{rule}}
	}
	if !in.Product.Found {
		return manual("PRODUCT_RULES_UNAVAILABLE")
	}
	checked := validation.Evaluate(in.Snapshot, validation.Product{
		Found: true, PolicyVersion: in.Product.PolicyVersion, PurposeTags: in.Product.Purposes, RequiredDocumentTypes: in.Product.Documents,
		MinAmountKopecks: in.Product.MinAmount, MaxAmountKopecks: in.Product.MaxAmount,
		MinTermMonths: int32(in.Product.MinTerm), MaxTermMonths: int32(in.Product.MaxTerm),
	})
	switch checked.Status {
	case model.CaseStatusNeedsInfo:
		return Result{Status: model.CaseStatusNeedsInfo, Outcome: "REQUEST_INFO", Rules: []string{checked.Reason},
			PublicReason: InfoMessage, RequiredFields: checked.RequiredFields, RequiredDocs: checked.RequiredDocumentTypes,
			Plan: checked.Plan, Requirements: checked.RequestedRequirements}
	case model.CaseStatusManualReview:
		result := manual(checked.Reason)
		result.PublicReason = checked.PublicReason
		return result
	}
	if !in.CompanyOK || !in.CreditOK {
		return manual("BUREAU_RESULT_UNAVAILABLE")
	}
	if !in.ScoreFound {
		return manual("SCORE_RESULT_UNAVAILABLE")
	}
	status := strings.TrimPrefix(in.CompanyStatus, "COMPANY_STATUS_")
	if status == "LIQUIDATING" || status == "LIQUIDATED" {
		return Result{Status: model.CaseStatusRejected, Outcome: "REJECT", Rules: []string{"COMPANY_LIQUIDATION"}, PublicReason: RejectMessage}
	}
	if in.OverdueDays >= 90 {
		return Result{Status: model.CaseStatusRejected, Outcome: "REJECT", Rules: []string{"OVERDUE_90_DAYS"}, PublicReason: RejectMessage}
	}
	if status != "ACTIVE" {
		return manual("COMPANY_STATUS_REVIEW")
	}
	switch in.Grade {
	case "E":
		return Result{Status: model.CaseStatusRejected, Outcome: "REJECT", Rules: []string{"GRADE_E"}, PublicReason: RejectMessage}
	case "C", "D":
		return manual("GRADE_REVIEW")
	case "A", "B":
	default:
		return manual("GRADE_UNKNOWN")
	}
	p := in.Product
	if in.Snapshot.AmountKopecks < p.MinAmount || in.Snapshot.AmountKopecks > p.MaxAmount ||
		int(in.Snapshot.TermMonths) < p.MinTerm || int(in.Snapshot.TermMonths) > p.MaxTerm {
		return manual("PRODUCT_RANGE_REVIEW")
	}
	var company struct {
		RegisteredAt string `json:"registered_at"`
		Revenue      *struct {
			Amount json.Number `json:"amount_kopecks"`
		} `json:"revenue_12m"`
	}
	if err := json.Unmarshal(in.Snapshot.CompanyJSON, &company); err != nil || company.Revenue == nil {
		return manual("COMPANY_FINANCIALS_REVIEW")
	}
	registered, err := time.Parse("2006-01-02", company.RegisteredAt)
	if err != nil {
		return manual("COMPANY_AGE_REVIEW")
	}
	if registered.AddDate(0, p.MinAgeMonths, 0).After(in.Now) {
		return manual("COMPANY_AGE_REVIEW")
	}
	revenue, err := company.Revenue.Amount.Int64()
	if err != nil || revenue <= 0 {
		return manual("COMPANY_FINANCIALS_REVIEW")
	}
	amountScaled := new(big.Int).Mul(big.NewInt(in.Snapshot.AmountKopecks), big.NewInt(10000))
	allowed := new(big.Int).Mul(big.NewInt(revenue), big.NewInt(int64(p.MaxAmountRevenueBP)))
	if amountScaled.Cmp(allowed) > 0 {
		return manual("AMOUNT_REVENUE_REVIEW")
	}
	if in.OverdueDays > p.MaxOverdueDays {
		return manual("OVERDUE_PRODUCT_REVIEW")
	}
	rate := p.GradeARateBP
	if in.Grade == "B" {
		rate = p.GradeBRateBP
	}
	return Result{Status: model.CaseStatusOffer, Outcome: "APPROVE", Rules: []string{"AUTO_OFFER"}, RateBP: rate}
}
