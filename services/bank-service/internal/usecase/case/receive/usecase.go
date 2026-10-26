package receive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

var ErrInvalidEvent = errors.New("invalid client event")
var innPattern = regexp.MustCompile(`^[0-9]{10}([0-9]{2})?$`)

type Input struct {
	InfoProvided              bool
	Cancelled                 bool
	CancelReason              string
	OfferAction               string
	OfferID                   uuid.UUID
	SignedAt                  time.Time
	RequestID                 uuid.UUID
	EventID                   uuid.UUID
	ApplicationID             uuid.UUID
	AggregateVersion          int64
	OrgID                     uuid.UUID
	ClientUserID              uuid.UUID
	ProductID                 uuid.UUID
	ProductVersion            int64
	RequirementsReferenceDate *time.Time
	AnsweredRequirementIDs    []string
	InvalidDocumentMetadata   bool
	CompanyINN                string
	CompanyName               string
	ProfileSchemaVersion      int32
	AssessmentInputJSON       []byte
	CompanyJSON               []byte
	FormJSON                  []byte
	DocumentsJSON             []byte
	AmountKopecks             int64
	TermMonths                int32
	PurposeTag                string
}

type Repository interface {
	Apply(context.Context, Input) (bool, error)
}

type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}

type UseCase struct {
	repository   Repository
	transactions TransactionManager
}

func New(repository Repository, transactions TransactionManager) *UseCase {
	return &UseCase{repository: repository, transactions: transactions}
}

func (uc *UseCase) Execute(ctx context.Context, event *eventspb.Event) (bool, error) {
	input, err := parse(event)
	if err != nil {
		return false, err
	}
	var applied bool
	err = uc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		var applyErr error
		applied, applyErr = uc.repository.Apply(txCtx, input)
		return applyErr
	})
	if err != nil {
		return false, fmt.Errorf("receive client event: %w", err)
	}
	return applied, nil
}

func parse(event *eventspb.Event) (Input, error) {
	if event == nil || event.GetAggregateType() != "APPLICATION" || event.GetAggregateVersion() <= 0 ||
		event.GetOccurredAt() == nil || event.GetOccurredAt().CheckValid() != nil ||
		strings.TrimSpace(event.GetCorrelationId()) == "" {
		return Input{}, ErrInvalidEvent
	}

	eventID, err := nonzeroUUID(event.GetEventId())
	if err != nil {
		return Input{}, fmt.Errorf("%w: event_id", ErrInvalidEvent)
	}

	applicationID, err := nonzeroUUID(event.GetAggregateId())
	if err != nil {
		return Input{}, fmt.Errorf("%w: aggregate_id", ErrInvalidEvent)
	}

	input := Input{EventID: eventID, ApplicationID: applicationID, AggregateVersion: event.GetAggregateVersion()}
	switch payload := event.GetPayload().(type) {
	case *eventspb.Event_ApplicationCancelled:
		if payload.ApplicationCancelled == nil || !utf8.ValidString(payload.ApplicationCancelled.GetReason()) ||
			utf8.RuneCountInString(payload.ApplicationCancelled.GetReason()) > 500 {
			return Input{}, fmt.Errorf("%w: cancellation reason", ErrInvalidEvent)
		}
		input.Cancelled = true
		input.CancelReason = payload.ApplicationCancelled.GetReason()
		return input, nil
	case *eventspb.Event_OfferAccepted:
		input.OfferAction = "OfferAccepted"
		if payload.OfferAccepted != nil {
			input.OfferID, err = nonzeroUUID(payload.OfferAccepted.GetOfferId())
		}
		if payload.OfferAccepted == nil || err != nil {
			return Input{}, fmt.Errorf("%w: offer_id", ErrInvalidEvent)
		}
		return input, nil
	case *eventspb.Event_OfferDeclined:
		input.OfferAction = "OfferDeclined"
		if payload.OfferDeclined != nil {
			input.OfferID, err = nonzeroUUID(payload.OfferDeclined.GetOfferId())
		}
		if payload.OfferDeclined == nil || err != nil {
			return Input{}, fmt.Errorf("%w: offer_id", ErrInvalidEvent)
		}
		return input, nil
	case *eventspb.Event_ContractSigned:
		input.OfferAction = "ContractSigned"
		if payload.ContractSigned == nil || payload.ContractSigned.GetSignedAt() == nil ||
			payload.ContractSigned.GetSignedAt().CheckValid() != nil {
			return Input{}, fmt.Errorf("%w: signed_at", ErrInvalidEvent)
		}
		input.OfferID, err = nonzeroUUID(payload.ContractSigned.GetOfferId())
		if err != nil {
			return Input{}, fmt.Errorf("%w: offer_id", ErrInvalidEvent)
		}
		input.SignedAt = payload.ContractSigned.GetSignedAt().AsTime()
		if input.SignedAt.IsZero() || input.SignedAt.Before(event.GetOccurredAt().AsTime().Add(-5*time.Minute)) ||
			input.SignedAt.After(event.GetOccurredAt().AsTime().Add(5*time.Minute)) {
			return Input{}, fmt.Errorf("%w: signed_at differs from event time", ErrInvalidEvent)
		}
		return input, nil
	case *eventspb.Event_ApplicationSubmitted, *eventspb.Event_InfoProvided:
	default:
		return Input{}, fmt.Errorf("%w: payload", ErrInvalidEvent)
	}
	submitted := event.GetApplicationSubmitted()
	provided := event.GetInfoProvided()
	if submitted == nil && provided == nil {
		return Input{}, fmt.Errorf("%w: payload", ErrInvalidEvent)
	}
	var company *commonpb.CompanySnapshot
	var form *commonpb.ApplicationForm
	var documentsInput []*commonpb.DocumentMetadata
	if submitted != nil {
		var orgErr, userErr, productErr error
		input.OrgID, orgErr = nonzeroUUID(submitted.GetOrgId())
		input.ClientUserID, userErr = nonzeroUUID(submitted.GetClientUserId())
		input.ProductID, productErr = nonzeroUUID(submitted.GetProductId())
		if orgErr != nil || userErr != nil || productErr != nil || submitted.GetProductVersion() <= 0 {
			return Input{}, fmt.Errorf("%w: identifiers", ErrInvalidEvent)
		}
		input.ProductVersion = submitted.GetProductVersion()
		company, form, documentsInput = submitted.GetCompany(), submitted.GetForm(), submitted.GetDocuments()
		if value := submitted.GetRequirementsReferenceDate(); value != "" {
			date, parseErr := time.Parse("2006-01-02", value)
			if parseErr != nil || date.Format("2006-01-02") != value ||
				date.After(event.GetOccurredAt().AsTime().UTC().Truncate(24*time.Hour)) {
				return Input{}, fmt.Errorf("%w: requirements_reference_date", ErrInvalidEvent)
			}
			input.RequirementsReferenceDate = &date
		}
		if company != nil && company.GetProfileSchemaVersion() == 2 && input.RequirementsReferenceDate == nil {
			return Input{}, fmt.Errorf("%w: missing requirements_reference_date", ErrInvalidEvent)
		}
	} else {
		input.InfoProvided = true
		input.RequestID, err = nonzeroUUID(provided.GetRequestId())
		if err != nil {
			return Input{}, fmt.Errorf("%w: request_id", ErrInvalidEvent)
		}
		company, form, documentsInput = provided.GetCompany(), provided.GetForm(), provided.GetDocuments()
		input.AnsweredRequirementIDs = append([]string(nil), provided.GetAnsweredRequirementIds()...)
	}
	if company == nil || form == nil {
		return Input{}, fmt.Errorf("%w: snapshots", ErrInvalidEvent)
	}
	if !innPattern.MatchString(company.GetInn()) || strings.TrimSpace(company.GetName()) == "" ||
		len(strings.TrimSpace(company.GetOkved())) < 2 || len(strings.TrimSpace(company.GetOkved())) > 16 ||
		!validDate(company.GetRegisteredAt()) || !validOptionalMoney(company.GetRevenue_12M(), false) ||
		!validOptionalMoney(company.GetProfit_12M(), true) || !validOptionalMoney(company.GetObligations(), false) ||
		!validMoney(form.GetAmount(), false) || form.GetAmount().GetAmountKopecks() <= 0 ||
		form.GetTermMonths() <= 0 {
		return Input{}, fmt.Errorf("%w: company or form", ErrInvalidEvent)
	}
	purpose := strings.TrimPrefix(form.GetPurpose().String(), "PURPOSE_TAG_")
	switch form.GetPurpose() {
	case commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL,
		commonpb.PurposeTag_PURPOSE_TAG_EQUIPMENT,
		commonpb.PurposeTag_PURPOSE_TAG_EXPANSION:
	default:
		return Input{}, fmt.Errorf("%w: purpose", ErrInvalidEvent)
	}
	companyJSON, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(company)
	if err != nil {
		return Input{}, fmt.Errorf("encode company snapshot: %w", err)
	}

	formJSON, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(form)
	if err != nil {
		return Input{}, fmt.Errorf("encode application snapshot: %w", err)
	}

	documents := make([]DocumentSnapshot, 0, len(documentsInput))
	for _, document := range documentsInput {
		if document == nil || document.GetUploadedAt() == nil || document.GetUploadedAt().CheckValid() != nil {
			if provided != nil && company.GetProfileSchemaVersion() == 2 {
				input.InvalidDocumentMetadata = true
				continue
			}
			return Input{}, fmt.Errorf("%w: document", ErrInvalidEvent)
		}
		if _, err := nonzeroUUID(document.GetDocumentId()); err != nil {
			if provided != nil && company.GetProfileSchemaVersion() == 2 {
				input.InvalidDocumentMetadata = true
				continue
			}
			return Input{}, fmt.Errorf("%w: document_id", ErrInvalidEvent)
		}
		documentType := strings.TrimPrefix(document.GetDocumentType().String(), "DOCUMENT_TYPE_")
		_, known := commonpb.DocumentType_name[int32(document.GetDocumentType())]
		if document.GetDocumentType() == commonpb.DocumentType_DOCUMENT_TYPE_UNSPECIFIED ||
			!known {
			if provided != nil && company.GetProfileSchemaVersion() == 2 {
				input.InvalidDocumentMetadata = true
				continue
			}
			return Input{}, fmt.Errorf("%w: document_type", ErrInvalidEvent)
		}
		if document.GetRequirementId() != "" || document.GetCoveragePeriod() != nil {
			coverage := document.GetCoveragePeriod()
			if document.GetRequirementId() == "" || coverage == nil || !validDate(coverage.GetFrom()) ||
				!validDate(coverage.GetTo()) || coverage.GetFrom() > coverage.GetTo() {
				if provided != nil && company.GetProfileSchemaVersion() == 2 {
					input.InvalidDocumentMetadata = true
					continue
				}
				return Input{}, fmt.Errorf("%w: document coverage", ErrInvalidEvent)
			}
		}
		documents = append(documents, DocumentSnapshot{
			DocumentID: document.GetDocumentId(), DocumentType: documentType,
			FileName: document.GetFileName(), SHA256: document.GetSha256(),
			UploadedAt: document.GetUploadedAt().AsTime().UTC(), IsMock: document.GetIsMock(),
			RequirementID: document.GetRequirementId(), CoveragePeriod: document.GetCoveragePeriod(),
		})
	}
	documentsJSON, err := json.Marshal(documents)
	if err != nil {
		return Input{}, fmt.Errorf("encode documents snapshot: %w", err)
	}
	input.CompanyINN, input.CompanyName = company.GetInn(), company.GetName()
	input.ProfileSchemaVersion = company.GetProfileSchemaVersion()
	input.CompanyJSON, input.FormJSON, input.DocumentsJSON = companyJSON, formJSON, documentsJSON
	input.AssessmentInputJSON = []byte(`{"schema_version":1,"preferences":{}}`)
	if submitted != nil && len(submitted.AssessmentInputJson) > 0 {
		input.AssessmentInputJSON = submitted.AssessmentInputJson
	}
	if provided != nil && len(provided.AssessmentInputJson) > 0 {
		input.AssessmentInputJSON = provided.AssessmentInputJson
	}
	if len(input.AssessmentInputJSON) > 256*1024 || !json.Valid(input.AssessmentInputJSON) {
		return Input{}, errors.New("invalid assessment input")
	}
	var submission assessment.Submission
	decoder := json.NewDecoder(bytes.NewReader(input.AssessmentInputJSON))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&submission); err != nil {
		return Input{}, fmt.Errorf("%w: malformed assessment input", ErrInvalidEvent)
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Input{}, fmt.Errorf("%w: trailing assessment input", ErrInvalidEvent)
	}
	if err = assessment.ValidateSubmission(submission, form.GetAmount().GetAmountKopecks()); err != nil {
		return Input{}, fmt.Errorf("%w: %w", ErrInvalidEvent, err)
	}
	input.AmountKopecks, input.TermMonths, input.PurposeTag = form.GetAmount().GetAmountKopecks(), form.GetTermMonths(), purpose
	return input, nil
}

type DocumentSnapshot struct {
	DocumentID     string               `json:"document_id"`
	DocumentType   string               `json:"document_type"`
	FileName       string               `json:"file_name"`
	SHA256         string               `json:"sha256"`
	UploadedAt     time.Time            `json:"uploaded_at"`
	IsMock         bool                 `json:"is_mock"`
	RequirementID  string               `json:"requirement_id,omitempty"`
	CoveragePeriod *commonpb.DatePeriod `json:"coverage_period,omitempty"`
}

func nonzeroUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, ErrInvalidEvent
	}
	return id, nil
}

func validMoney(value *commonpb.Money, allowNegative bool) bool {
	return value != nil && value.GetCurrency() == "RUB" && (allowNegative || value.GetAmountKopecks() >= 0)
}

func validOptionalMoney(value *commonpb.Money, allowNegative bool) bool {
	return value == nil || validMoney(value, allowNegative)
}

func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}
