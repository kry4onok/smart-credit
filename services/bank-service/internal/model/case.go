package model

import (
	"time"

	"github.com/google/uuid"
)

type CaseStatus string

const (
	CaseStatusSubmitted     CaseStatus = "SUBMITTED"
	CaseStatusValidation    CaseStatus = "VALIDATION"
	CaseStatusNeedsInfo     CaseStatus = "NEEDS_INFO"
	CaseStatusEnrichment    CaseStatus = "ENRICHMENT"
	CaseStatusScoring       CaseStatus = "SCORING"
	CaseStatusDecision      CaseStatus = "DECISION"
	CaseStatusManualReview  CaseStatus = "MANUAL_REVIEW"
	CaseStatusOffer         CaseStatus = "OFFER"
	CaseStatusCounterOffer  CaseStatus = "COUNTER_OFFER"
	CaseStatusOfferAccepted CaseStatus = "OFFER_ACCEPTED"
	CaseStatusSigning       CaseStatus = "SIGNING"
	CaseStatusDisbursed     CaseStatus = "DISBURSED"
	CaseStatusRejected      CaseStatus = "REJECTED"
	CaseStatusCancelled     CaseStatus = "CANCELLED"
	CaseStatusExpired       CaseStatus = "EXPIRED"
)

type CaseListItem struct {
	ID                     uuid.UUID
	Status                 CaseStatus
	ProductID              uuid.UUID
	RequestedAmountKopecks int64
	CompanyName            string
	AssigneeID             *uuid.UUID
	CreatedAt              time.Time
	Version                int64
}

type Case struct {
	ID             uuid.UUID
	Status         CaseStatus
	ProductID      uuid.UUID
	ProductName    string
	ProductVersion int64
	Company        Company
	Application    SubmittedApplication
	TriggeredRules []string
	Recommendation *CaseRecommendation
	AssigneeID     *uuid.UUID
	PublicReason   *string
	AuditVersion   int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}

type Company struct {
	INN                string
	Name               string
	OKVED              string
	RegisteredAt       time.Time
	Revenue12MKopecks  int64
	Profit12MKopecks   int64
	ObligationsKopecks int64
}

type SubmittedApplication struct {
	AmountKopecks int64
	TermMonths    int
	Purpose       PurposeTag
	DocumentTypes []DocumentType
}

type PurposeTag string

type DocumentType string

type CaseRecommendation string

type CaseAction string

const (
	CaseActionView   CaseAction = "VIEW"
	CaseActionTake   CaseAction = "TAKE"
	CaseActionDecide CaseAction = "DECIDE"
)
