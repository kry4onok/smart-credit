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
	Documents      []DocumentMetadata
	Checks         *Checks
	Score          *ScoreResult
	TriggeredRules []string
	Recommendation *CaseRecommendation
	AssigneeID     *uuid.UUID
	InfoRequest    *InfoRequest
	Offer          *Offer
	PublicReason   *string
	AuditVersion   int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}

type DocumentMetadata struct {
	ID           uuid.UUID `json:"document_id"`
	DocumentType string    `json:"document_type"`
	FileName     string    `json:"file_name"`
	SHA256       string    `json:"sha256"`
	UploadedAt   time.Time `json:"uploaded_at"`
	IsMock       bool      `json:"is_mock"`
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

type Checks struct {
	CompanyStatus    CompanyStatus
	RegisteredAt     *time.Time
	OKVED            *string
	TotalDebtKopecks int64
	OverdueCount12M  int
	MaxOverdueDays   int
	BureauError      *string
}

type CompanyStatus string

type ScoreResult struct {
	AssessmentStatus     string
	Score                *int
	Grade                ScoreGrade
	Reasons              []ScoreReason
	DataIssues           []ScoreDataIssue
	FactorContributions  []ScoreFactorContribution
	ModelVersion         string
	FeatureSchemaVersion string
	BaselineScore        *int
	ConfigChecksum       *string
	AttemptID            *uuid.UUID
	InputRevision        *int64
	AsOfDate             *time.Time
}

type ScoreDataIssue struct {
	Code      string
	FieldPath string
}

type ScoreFactorContribution struct {
	FactorName  string
	Points      int
	Description string
}

type ScoreReason struct {
	Code         ScoreReasonCode
	Description  string
	Direction    ScoreReasonDirection
	Contribution int
}

type ScoreGrade string

type ScoreReasonCode string

type ScoreReasonDirection string

type InfoRequest struct {
	ID                    uuid.UUID
	Message               string
	RequiredFields        []InfoField
	RequiredDocumentTypes []DocumentType
	CreatedAt             time.Time
}

type InfoField string

type Offer struct {
	ID             uuid.UUID
	Kind           OfferKind
	ProductID      uuid.UUID
	ProductVersion int64
	ProductName    string
	AmountKopecks  int64
	TermMonths     int
	RateBPS        int
	ExpiresAt      time.Time
	Explanation    string
}

type OfferKind string

type PurposeTag string

type DocumentType string

type CaseRecommendation string

type CaseAction string

const (
	CaseActionView   CaseAction = "VIEW"
	CaseActionTake   CaseAction = "TAKE"
	CaseActionDecide CaseAction = "DECIDE"
)
