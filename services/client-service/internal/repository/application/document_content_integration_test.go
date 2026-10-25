package application

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	getdocumentcontent "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/document/content"
	"github.com/google/uuid"
)

func TestDocumentContentRequiresSubmittedApplicationIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	original := []byte("Демо-выписка\r\nОстаток: 1000.00\r\n")
	document, err := f.repo.AddDocument(t.Context(), model.AddApplicationDocumentParams{
		OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
		IdempotencyKey: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "Выписка.txt", Content: original,
	})
	if err != nil {
		t.Fatal(err)
	}
	params := model.GetDocumentContentParams{ApplicationID: f.draft.ID, DocumentID: document.ID}
	if _, err := f.repo.GetDocumentContent(t.Context(), params); !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("draft document is accessible: err=%v", err)
	}
	// A timestamp alone must not open access to a draft.
	if _, err := f.db.Exec(t.Context(), "UPDATE applications SET submitted_at = $2 WHERE id = $1", f.draft.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.GetDocumentContent(t.Context(), params); !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("draft with timestamp is accessible: err=%v", err)
	}
	f.submit(t)
	usecase := getdocumentcontent.New(f.repo)
	got, err := usecase.Execute(t.Context(), params)
	if err != nil || !bytes.Equal(got.Content, original) || got.FileName != document.FileName || got.ContentType != "text/plain; charset=utf-8" {
		t.Fatalf("stored content=%+v, err=%v", got, err)
	}
	otherParams := f.params
	otherParams.IdempotencyID = uuid.New()
	other, err := f.repo.Create(t.Context(), otherParams)
	if err != nil {
		t.Fatal(err)
	}
	otherFixture := applicationFixture{db: f.db, repo: f.repo, params: otherParams, draft: other}
	otherFixture.submit(t)
	for _, target := range []model.GetDocumentContentParams{
		{ApplicationID: other.ID, DocumentID: document.ID},
		{ApplicationID: uuid.New(), DocumentID: document.ID},
		{ApplicationID: f.draft.ID, DocumentID: uuid.New()},
	} {
		if _, err := usecase.Execute(t.Context(), target); !errors.Is(err, domain.ErrDocumentNotFound) {
			t.Fatalf("missing or mismatched document accepted: params=%+v, err=%v", target, err)
		}
	}
	if _, err := f.db.Exec(t.Context(), "UPDATE applications SET status = 'CLOSED' WHERE id = $1", f.draft.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := usecase.Execute(t.Context(), params); err != nil || !bytes.Equal(got.Content, original) {
		t.Fatalf("closed submitted application lost document: err=%v", err)
	}
}

func TestDocumentContentStorageFailuresIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	f.submit(t)
	// Existing fixture uploads live in the same local volume as real files.
	card, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || len(card.Documents) == 0 {
		t.Fatalf("submitted documents=%v, err=%v", card.Documents, err)
	}
	documentID := card.Documents[0].ID
	params := model.GetDocumentContentParams{ApplicationID: f.draft.ID, DocumentID: documentID}
	key := filepath.Join(f.draft.ID.String(), documentID.String())
	path := filepath.Join(f.repo.storageDir, key)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		set  func(*testing.T)
	}{
		{"missing file", func(t *testing.T) {
			t.Helper()
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}},
		{"directory instead of file", func(t *testing.T) {
			t.Helper()
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink outside volume", func(t *testing.T) {
			t.Helper()
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "private.txt")
			if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
		}},
		{"unsafe storage key", func(t *testing.T) {
			t.Helper()
			if _, err := f.db.Exec(t.Context(), "UPDATE application_documents SET storage_key = '../private.txt' WHERE id = $1", documentID); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.RemoveAll(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Exec(t.Context(), "UPDATE application_documents SET storage_key = $2 WHERE id = $1", documentID, filepath.ToSlash(key)); err != nil {
				t.Fatal(err)
			}
			tc.set(t)
			if got, err := f.repo.GetDocumentContent(t.Context(), params); !errors.Is(err, domain.ErrDocumentNotFound) || len(got.Content) != 0 {
				t.Fatalf("unsafe or missing file was served: content=%+v, err=%v", got, err)
			}
		})
	}
}

func TestDocumentContentDetectsTruncatedFileIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	f.submit(t)
	card, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || len(card.Documents) == 0 {
		t.Fatalf("documents=%v, err=%v", card.Documents, err)
	}
	documentID := card.Documents[0].ID
	path := filepath.Join(f.repo.storageDir, f.draft.ID.String(), documentID.String())
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.GetDocumentContent(t.Context(), model.GetDocumentContentParams{ApplicationID: f.draft.ID, DocumentID: documentID}); err == nil || errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("truncated content did not produce a storage error: err=%v", err)
	}
}
