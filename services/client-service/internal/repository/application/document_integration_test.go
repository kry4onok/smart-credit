package application

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	documentusecase "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/document"
	"github.com/google/uuid"
)

func TestDocumentUploadPreservesOriginalIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	content := []byte("Демо-выписка\r\nВходящий остаток: 1000.00\r\nИсходящий остаток: 1500.00\r\n")
	storage := t.TempDir()
	repo := New(Adapt(f.db), storage)
	usecase := documentusecase.New(repo)
	params := model.AddApplicationDocumentParams{
		OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
		IdempotencyKey: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "statement.txt", Content: content,
	}
	document, err := usecase.Execute(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(storage, filepath.FromSlash(document.StorageKey)))
	if err != nil || !bytes.Equal(stored, content) {
		t.Fatalf("original bytes were not preserved: err=%v", err)
	}
	hash := sha256.Sum256(content)
	if document.SizeBytes != int64(len(content)) || document.Sha256 != hex.EncodeToString(hash[:]) || !document.IsMock {
		t.Fatalf("unexpected metadata: %+v", document)
	}
	replay, err := usecase.Execute(t.Context(), params)
	if err != nil || replay.ID != document.ID {
		t.Fatalf("idempotent upload=%+v, err=%v", replay, err)
	}
	params.FileName, params.IdempotencyKey = "renamed.pdf", uuid.New()
	if _, err := usecase.Execute(t.Context(), params); !errors.Is(err, domain.ErrInvalidDocumentUpload) {
		t.Fatalf("renamed text accepted as PDF: err=%v", err)
	}
	var documents, keys, outbox int
	var version int64
	if err := f.db.QueryRow(t.Context(), `SELECT version,
		(SELECT count(*) FROM application_documents WHERE application_id = $1),
		(SELECT count(*) FROM idempotency_keys WHERE route LIKE '%/documents'),
		(SELECT count(*) FROM outbox)
		FROM applications WHERE id = $1`, f.draft.ID).Scan(&version, &documents, &keys, &outbox); err != nil {
		t.Fatal(err)
	}
	if documents != 1 || keys != 1 || outbox != 0 || version != f.draft.Version+1 {
		t.Fatalf("upload/replay/rejection: documents=%d, keys=%d, outbox=%d, version=%d", documents, keys, outbox, version)
	}
	card, err := repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || len(card.Documents) != 1 || card.Documents[0].ID != document.ID || card.Documents[0].Sha256 != document.Sha256 {
		t.Fatalf("uploaded document missing from card: %+v, err=%v", card.Documents, err)
	}
	files, err := os.ReadDir(filepath.Join(storage, f.draft.ID.String()))
	if err != nil || len(files) != 1 {
		t.Fatalf("upload/replay/rejection left extra files: files=%d, err=%v", len(files), err)
	}
}
