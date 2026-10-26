package application

import (
	"errors"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

func TestSubmissionNamesAllMissingInputs(t *testing.T) {
	snapshot := submitSnapshot{company: companySnapshotJSON{RegisteredAt: "2020-01-01"},
		product: productSnapshotJSON{RequiredDocumentTypes: []string{"BANK_STATEMENT", "BUSINESS_PLAN"}}}
	err := validateSubmitSnapshot(snapshot)
	var validation *domain.ValidationError
	if !errors.Is(err, domain.ErrInvalidApplication) || !errors.As(err, &validation) {
		t.Fatal("missing submission data was not reported")
	}
	joined := strings.Join(validation.Issues, " ")
	for _, want := range []string{"сумму", "срок", "цель", "выписка", "бизнес-план"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing issue: %s", want)
		}
	}
	snapshot.application.Form = &model.ApplicationForm{AmountKopecks: 10000, TermMonths: 12}
	snapshot.product.MinAmount.AmountKopecks, snapshot.product.MaxAmount.AmountKopecks = 100, 100000
	snapshot.product.MinTermMonths, snapshot.product.MaxTermMonths = 1, 36
	snapshot.documents = []model.ApplicationDocument{{DocumentType: "BANK_STATEMENT"}, {DocumentType: "BUSINESS_PLAN"}}
	err = validateSubmitSnapshot(snapshot)
	if !errors.As(err, &validation) || len(validation.Issues) != 1 || !strings.Contains(validation.Issues[0], "цель") {
		t.Fatal("validation incorrectly reports already filled fields")
	}
	snapshot.application.Form.Purpose = "WORKING_CAPITAL"
	snapshot.product.PurposeTags = []string{"WORKING_CAPITAL"}
	if err := validateSubmitSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestInfoNamesMissingDocumentsAndKeepsAllExistingTypes(t *testing.T) {
	_, err := selectInfoDocuments(nil, []string{"BANK_STATEMENT", "BUSINESS_PLAN"}, model.ProvideInfoParams{})
	var validation *domain.ValidationError
	if !errors.Is(err, domain.ErrInvalidInfoResponse) || !errors.As(err, &validation) || len(validation.Issues) != 2 {
		t.Fatal("missing bank-request documents were not named")
	}
	documents := []model.ApplicationDocument{{DocumentType: "BANK_STATEMENT"}, {DocumentType: "BUSINESS_PLAN"}}
	selected, err := selectInfoDocuments(documents, []string{"BANK_STATEMENT"}, model.ProvideInfoParams{})
	if err != nil || len(selected) != 2 {
		t.Fatal("answer dropped previously attached document types")
	}
}
