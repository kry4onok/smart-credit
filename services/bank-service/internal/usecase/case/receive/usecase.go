package receive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

var ErrInvalidEvent = errors.New("invalid ApplicationSubmitted event")
var innPattern = regexp.MustCompile(`^[0-9]{10}([0-9]{2})?$`)

type Input struct {
	EventID          uuid.UUID
	ApplicationID    uuid.UUID
	AggregateVersion int64
	OrgID            uuid.UUID
	ClientUserID     uuid.UUID
	ProductID        uuid.UUID
	ProductVersion   int64
	CompanyINN       string
	CompanyName      string
	CompanyJSON      []byte
	FormJSON         []byte
	DocumentsJSON    []byte
	AmountKopecks    int64
	TermMonths       int32
	PurposeTag       string
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
		return false, fmt.Errorf("receive ApplicationSubmitted: %w", err)
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

	submitted := event.GetApplicationSubmitted()
	if submitted == nil || submitted.GetCompany() == nil || submitted.GetForm() == nil {
		return Input{}, fmt.Errorf("%w: payload", ErrInvalidEvent)
	}

	orgID, orgErr := nonzeroUUID(submitted.GetOrgId())
	clientUserID, userErr := nonzeroUUID(submitted.GetClientUserId())
	productID, productErr := nonzeroUUID(submitted.GetProductId())
	if orgErr != nil || userErr != nil || productErr != nil || submitted.GetProductVersion() <= 0 {
		return Input{}, fmt.Errorf("%w: identifiers", ErrInvalidEvent)
	}

	company := submitted.GetCompany()
	form := submitted.GetForm()
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

	documents := make([]documentSnapshot, 0, len(submitted.GetDocuments()))
	for _, document := range submitted.GetDocuments() {
		if document == nil || document.GetUploadedAt() == nil || document.GetUploadedAt().CheckValid() != nil {
			return Input{}, fmt.Errorf("%w: document", ErrInvalidEvent)
		}
		if _, err := nonzeroUUID(document.GetDocumentId()); err != nil {
			return Input{}, fmt.Errorf("%w: document_id", ErrInvalidEvent)
		}
		var documentType string
		switch document.GetDocumentType() {
		case commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT:
			documentType = "BANK_STATEMENT"
		case commonpb.DocumentType_DOCUMENT_TYPE_BUSINESS_PLAN:
			documentType = "BUSINESS_PLAN"
		default:
			return Input{}, fmt.Errorf("%w: document_type", ErrInvalidEvent)
		}
		documents = append(documents, documentSnapshot{
			DocumentID: document.GetDocumentId(), DocumentType: documentType,
			FileName: document.GetFileName(), SHA256: document.GetSha256(),
			UploadedAt: document.GetUploadedAt().AsTime().UTC(), IsMock: document.GetIsMock(),
		})
	}
	documentsJSON, err := json.Marshal(documents)
	if err != nil {
		return Input{}, fmt.Errorf("encode documents snapshot: %w", err)
	}
	return Input{
		EventID: eventID, ApplicationID: applicationID, AggregateVersion: event.GetAggregateVersion(),
		OrgID: orgID, ClientUserID: clientUserID, ProductID: productID,
		ProductVersion: submitted.GetProductVersion(), CompanyINN: company.GetInn(),
		CompanyName: company.GetName(), CompanyJSON: companyJSON, FormJSON: formJSON,
		DocumentsJSON: documentsJSON, AmountKopecks: form.GetAmount().GetAmountKopecks(),
		TermMonths: form.GetTermMonths(), PurposeTag: purpose,
	}, nil
}

type documentSnapshot struct {
	DocumentID   string    `json:"document_id"`
	DocumentType string    `json:"document_type"`
	FileName     string    `json:"file_name"`
	SHA256       string    `json:"sha256"`
	UploadedAt   time.Time `json:"uploaded_at"`
	IsMock       bool      `json:"is_mock"`
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
