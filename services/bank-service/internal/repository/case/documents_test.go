package caserepository

import (
	"testing"

	"github.com/google/uuid"
)

func TestDecodeCaseDocumentsAcceptsEventAndDemoSeedIDs(t *testing.T) {
	first := uuid.New()
	second := uuid.New()
	raw := []byte(`[{"document_id":"` + first.String() + `","document_type":"BANK_STATEMENT","file_name":"first.txt","sha256":"abc","uploaded_at":"2026-10-25T12:00:00Z","is_mock":true},{"id":"` + second.String() + `","document_type":"BUSINESS_PLAN","file_name":"second.pdf","sha256":"def","uploaded_at":"2026-10-25T12:00:00Z","is_mock":true}]`)
	documents, err := decodeCaseDocuments(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 2 || documents[0].ID != first || documents[1].ID != second ||
		documents[0].DocumentType != "BANK_STATEMENT" || documents[1].FileName != "second.pdf" {
		t.Fatalf("decoded documents = %+v", documents)
	}
}
