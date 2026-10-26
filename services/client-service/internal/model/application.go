package model

import (
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"

	"github.com/google/uuid"
)

type Application struct {
	PurposeDescription        string                     `json:"purpose_description,omitempty"`
	RequirementsReferenceDate *time.Time                 `json:"requirements_reference_date,omitempty"`
	RequirementsPlan          *commonpb.RequirementsPlan `json:"requirements_plan,omitempty"`
	ID                        uuid.UUID                  `json:"id"`
	ProductID                 uuid.UUID                  `json:"product_id"`
	ProductVersion            int64                      `json:"product_version"`
	Company                   Company                    `json:"company"`
	Form                      *ApplicationForm           `json:"form,omitempty"`
	Status                    string                     `json:"status"`
	StageLabel                string                     `json:"stage_label"`
	NextAction                string                     `json:"next_action"`
	AllowedActions            []string                   `json:"allowed_actions"`
	PendingAction             *string                    `json:"pending_action,omitempty"`
	Documents                 []ApplicationDocument      `json:"documents"`
	InfoRequest               *ApplicationInfoRequest    `json:"info_request,omitempty"`
	Offer                     *ApplicationOffer          `json:"offer,omitempty"`
	PublicReason              *string                    `json:"public_reason,omitempty"`
	CreatedAt                 time.Time                  `json:"created_at"`
	UpdatedAt                 time.Time                  `json:"updated_at"`
	Version                   int64                      `json:"version"`
}

type ApplicationListItem struct {
	ID            uuid.UUID
	ProductID     uuid.UUID
	AmountKopecks *int64
	Status        string
	NextAction    string
	CreatedAt     time.Time
}

type ApplicationForm struct {
	AmountKopecks      int64  `json:"amount_kopecks"`
	TermMonths         int    `json:"term_months"`
	Purpose            string `json:"purpose"`
	PurposeDescription string `json:"purpose_description,omitempty"`
}

type ApplicationDocument struct {
	RequirementID *string    `json:"requirement_id,omitempty"`
	CoverageFrom  *time.Time `json:"coverage_from,omitempty"`
	CoverageTo    *time.Time `json:"coverage_to,omitempty"`
	ID            uuid.UUID  `json:"id"`
	DocumentType  string     `json:"document_type"`
	FileName      string     `json:"file_name"`
	Sha256        string     `json:"sha256"`
	IsMock        bool       `json:"is_mock"`
	UploadedAt    time.Time  `json:"uploaded_at"`
	StorageKey    string     `json:"-"`
	SizeBytes     int64      `json:"-"`
}

type ApplicationInfoRequest struct {
	Requirements          []*commonpb.Requirement `json:"requirements,omitempty"`
	ID                    uuid.UUID               `json:"id"`
	Message               string                  `json:"message"`
	RequiredFields        []string                `json:"required_fields"`
	RequiredDocumentTypes []string                `json:"required_document_types"`
	CreatedAt             time.Time               `json:"created_at"`
}

type ApplicationOffer struct {
	ID             uuid.UUID `json:"id"`
	Kind           string    `json:"kind"`
	ProductID      uuid.UUID `json:"product_id"`
	ProductVersion int64     `json:"product_version"`
	ProductName    string    `json:"product_name"`
	AmountKopecks  int64     `json:"amount_kopecks"`
	TermMonths     int       `json:"term_months"`
	RateBps        int       `json:"rate_bps"`
	Explanation    string    `json:"explanation"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type CreateApplicationParams struct {
	OrgID         uuid.UUID
	UserID        uuid.UUID
	ProductID     uuid.UUID
	IdempotencyID uuid.UUID
}

type ApplicationPatch struct {
	Company            *Company
	AmountKopecks      *int64
	AmountCurrency     string
	TermMonths         *int
	Purpose            *string
	PurposeDescription *string
}

type DeleteApplicationParams struct {
	OrgID           uuid.UUID
	ApplicationID   uuid.UUID
	ExpectedVersion int64
}

type UpdateApplicationParams struct {
	OrgID           uuid.UUID
	ApplicationID   uuid.UUID
	ExpectedVersion int64
	Patch           ApplicationPatch
}

type AddApplicationDocumentParams struct {
	RequirementID  *string
	CoverageFrom   *time.Time
	CoverageTo     *time.Time
	OrgID          uuid.UUID
	UserID         uuid.UUID
	ApplicationID  uuid.UUID
	IdempotencyKey uuid.UUID
	DocumentType   string
	FileName       string
	Content        []byte
}

type SubmitApplicationParams struct {
	OrgID         uuid.UUID
	UserID        uuid.UUID
	ApplicationID uuid.UUID
	IdempotencyID uuid.UUID
}

type ProvideInfoParams struct {
	OrgID              uuid.UUID
	UserID             uuid.UUID
	ApplicationID      uuid.UUID
	RequestID          uuid.UUID
	IdempotencyID      uuid.UUID
	Company            *Company
	Fields             *ApplicationPatch
	DocumentIDs        []uuid.UUID
	DocumentsSpecified bool
	RequestHash        []byte
}

type AcceptOfferParams struct {
	OrgID         uuid.UUID
	UserID        uuid.UUID
	ApplicationID uuid.UUID
	OfferID       uuid.UUID
	IdempotencyID uuid.UUID
	RequestHash   []byte
}

type DeclineOfferParams struct {
	OrgID         uuid.UUID
	UserID        uuid.UUID
	ApplicationID uuid.UUID
	OfferID       uuid.UUID
	IdempotencyID uuid.UUID
	RequestHash   []byte
}

type SignContractParams struct {
	OrgID         uuid.UUID
	UserID        uuid.UUID
	ApplicationID uuid.UUID
	OfferID       uuid.UUID
	IdempotencyID uuid.UUID
	RequestHash   []byte
}

type ActionAccepted struct {
	ApplicationID uuid.UUID `json:"application_id"`
	PendingAction string    `json:"pending_action"`
}

type CancelApplicationParams struct {
	OrgID         uuid.UUID
	UserID        uuid.UUID
	ApplicationID uuid.UUID
	IdempotencyID uuid.UUID
	Reason        string
	RequestHash   []byte
}
