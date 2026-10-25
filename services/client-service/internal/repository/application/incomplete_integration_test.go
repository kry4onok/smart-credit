package application

import (
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestIncompleteSubmissionStaysDraftIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	amount := int64(10000)
	if _, err := f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: f.draft.ID,
		ExpectedVersion: f.draft.Version, Patch: model.ApplicationPatch{AmountKopecks: &amount, AmountCurrency: "RUB"}}); err != nil {
		t.Fatal(err)
	}
	params := model.SubmitApplicationParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyID: uuid.New()}
	_, err := f.repo.Submit(t.Context(), params)
	var validation *domain.ValidationError
	if !errors.Is(err, domain.ErrInvalidApplication) || !errors.As(err, &validation) {
		t.Fatal("incomplete submission accepted or details lost")
	}
	detail := strings.Join(validation.Issues, " ")
	if strings.Contains(detail, "сумму") || !strings.Contains(detail, "срок") || !strings.Contains(detail, "цель") || !strings.Contains(detail, "выписка") || !strings.Contains(detail, "бизнес-план") {
		t.Fatal("validation did not identify the actual missing data")
	}
	var status string
	var outbox, reserved int
	if err := f.db.QueryRow(t.Context(), `SELECT status, (SELECT count(*) FROM outbox),
	(SELECT count(*) FROM idempotency_keys WHERE idempotency_key=$2) FROM applications WHERE id=$1`, f.draft.ID, params.IdempotencyID).Scan(&status, &outbox, &reserved); err != nil || status != "DRAFT" || outbox != 0 || reserved != 0 {
		t.Fatal("rejected submission changed application state or queued an event")
	}
	if result := f.submit(t); result.Status != "SUBMITTED" {
		t.Fatal("completed draft could not be submitted")
	}
}

func TestInfoAfterUploadRetainsFullDocumentSnapshotIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	f.submit(t)
	requestID := uuid.New()
	f.project(t, 1, &eventspb.ApplicationPublicStatePublished{Status: eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO, StageLabel: "Нужны данные",
		ActiveInfoRequest: &commonpb.InfoRequest{RequestId: requestID.String(), CreatedAt: timestamppb.Now(), Message: "Уточните срок и обязательства",
			RequiredFields:        []commonpb.InfoField{commonpb.InfoField_INFO_FIELD_COMPANY_OBLIGATIONS, commonpb.InfoField_INFO_FIELD_APPLICATION_TERM_MONTHS},
			RequiredDocumentTypes: []commonpb.DocumentType{commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT}}})
	hash := sha256.Sum256([]byte("complete response"))
	params := model.ProvideInfoParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
		RequestID: requestID, IdempotencyID: uuid.New(), RequestHash: hash[:]}
	if _, err := f.repo.ProvideInfo(t.Context(), params); !errors.Is(err, domain.ErrInvalidInfoResponse) {
		t.Fatal("missing requested fields were accepted")
	}
	current, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || current.Status != "NEEDS_INFO" || current.PendingAction != nil {
		t.Fatal("incomplete answer left NEEDS_INFO")
	}
	if _, err := f.repo.AddDocument(t.Context(), model.AddApplicationDocumentParams{OrgID: f.params.OrgID, UserID: f.params.UserID,
		ApplicationID: f.draft.ID, IdempotencyKey: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "updated.txt", Content: []byte("updated statement")}); err != nil {
		t.Fatal(err)
	}
	// Re-read as after a browser reload; document IDs need not be remembered by the UI.
	current, err = f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || len(current.Documents) != 3 {
		t.Fatal("uploaded attachment was not persisted")
	}
	term := 18
	params.Company, params.Fields = &current.Company, &model.ApplicationPatch{TermMonths: &term}
	if _, err := f.repo.ProvideInfo(t.Context(), params); err != nil {
		t.Fatal(err)
	}
	info := f.event(t, "InfoProvided").GetInfoProvided()
	if len(info.GetDocuments()) != 3 || info.GetForm().GetTermMonths() != 18 {
		t.Fatal("answer lost stored documents or the updated field")
	}
	hasPlan := false
	for _, document := range info.GetDocuments() {
		if document.GetDocumentType() == commonpb.DocumentType_DOCUMENT_TYPE_BUSINESS_PLAN {
			hasPlan = true
		}
	}
	if !hasPlan {
		t.Fatal("answer discarded the original business plan")
	}
}
