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
