package application

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	listapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/worker/documentcleanup"
	"github.com/google/uuid"
)

func deleteParams(f applicationFixture, version int64) model.DeleteApplicationParams {
	return model.DeleteApplicationParams{OrgID: f.params.OrgID, ApplicationID: f.draft.ID, ExpectedVersion: version}
}
func readyDraft(t *testing.T) applicationFixture {
	t.Helper()
	f := newApplicationFixture(t)
	_, err := f.db.Exec(t.Context(), `UPDATE applications SET requested_amount_kopecks=10000,requested_term_months=12,purpose_tag='WORKING_CAPITAL',product_snapshot=jsonb_set(product_snapshot,'{required_document_types}','[]') WHERE id=$1`, f.draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func TestDeleteDraftAndCleanupIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	doc, err := f.repo.AddDocument(t.Context(), model.AddApplicationDocumentParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyKey: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "statement.txt", Content: []byte("statement")})
	if err != nil {
		t.Fatal(err)
	}
	current, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(current.AllowedActions, "DELETE_DRAFT") {
		t.Fatal("draft cannot be deleted")
	}
	if err = f.repo.Delete(t.Context(), deleteParams(f, current.Version)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID); !errors.Is(err, domain.ErrApplicationNotFound) {
		t.Fatalf("get after deletion: %v", err)
	}
	if err = f.repo.Delete(t.Context(), deleteParams(f, current.Version)); !errors.Is(err, domain.ErrApplicationNotFound) {
		t.Fatalf("repeat delete: %v", err)
	}
	if _, err = f.repo.Create(t.Context(), f.params); !errors.Is(err, domain.ErrApplicationNotFound) {
		t.Fatalf("creation replay resurrected draft: %v", err)
	}
	items, err := f.repo.List(t.Context(), listapplication.RepositoryQuery{OrgID: f.params.OrgID, Limit: 10})
	if err != nil || len(items) != 0 {
		t.Fatalf("list=%v err=%v", items, err)
	}
	var count int
	if err = f.db.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM application_documents)+(SELECT count(*) FROM timeline_entries)+(SELECT count(*) FROM outbox)`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("remaining dependencies=%d err=%v", count, err)
	}
	path := filepath.Join(f.repo.storageDir, filepath.FromSlash(doc.StorageKey))
	if _, err = os.Stat(path); err != nil {
		t.Fatal("files removed before durable cleanup", err)
	}
	worker := documentcleanup.New(f.db, f.repo.storageDir)
	if done, err := worker.CleanOne(t.Context()); err != nil || !done {
		t.Fatalf("cleanup done=%v err=%v", done, err)
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file still exists: %v", err)
	}
	if done, err := worker.CleanOne(t.Context()); err != nil || done {
		t.Fatalf("second cleanup done=%v err=%v", done, err)
	}
}
func TestDeleteDraftGuardsIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	for _, tc := range []struct {
		name   string
		params model.DeleteApplicationParams
		want   error
	}{
		{"wrong organization", model.DeleteApplicationParams{OrgID: uuid.New(), ApplicationID: f.draft.ID, ExpectedVersion: 1}, domain.ErrApplicationNotFound},
		{"wrong version", deleteParams(f, 2), domain.ErrApplicationVersionConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := f.repo.Delete(t.Context(), tc.params); !errors.Is(err, tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	for _, status := range []string{"SUBMITTED", "UNDER_REVIEW", "NEEDS_INFO", "OFFER_AVAILABLE", "FORMALIZING", "DISBURSED", "REJECTED", "CLOSED"} {
		t.Run(status, func(t *testing.T) {
			f := readyDraft(t)
			_, err := f.db.Exec(t.Context(), `UPDATE applications SET status=$2,submitted_at=now(),client_event_version=1 WHERE id=$1`, f.draft.ID, status)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.repo.Delete(t.Context(), deleteParams(f, 1)); !errors.Is(err, domain.ErrApplicationNotDraft) {
				t.Fatalf("error=%v", err)
			}
			got, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
			if err != nil || slices.Contains(got.AllowedActions, "DELETE_DRAFT") {
				t.Fatalf("actions=%v err=%v", got.AllowedActions, err)
			}
		})
	}
}
func TestDeleteDraftRacesIntegration(t *testing.T) {
	for _, action := range []string{"submit", "update", "upload"} {
		t.Run(action, func(t *testing.T) {
			f := readyDraft(t)
			if action == "upload" {
				if _, err := f.db.Exec(t.Context(), `UPDATE applications SET product_snapshot=jsonb_set(product_snapshot,'{required_document_types}','["BANK_STATEMENT"]') WHERE id=$1`, f.draft.ID); err != nil {
					t.Fatal(err)
				}
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			go func() { <-start; results <- f.repo.Delete(t.Context(), deleteParams(f, 1)) }()
			go func() {
				<-start
				var err error
				switch action {
				case "submit":
					_, err = f.repo.Submit(t.Context(), model.SubmitApplicationParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyID: uuid.New()})
				case "update":
					term := 18
					_, err = f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: f.draft.ID, ExpectedVersion: 1, Patch: model.ApplicationPatch{TermMonths: &term}})
				case "upload":
					_, err = f.repo.AddDocument(t.Context(), model.AddApplicationDocumentParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyKey: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "statement.txt", Content: []byte("statement")})
				}
				results <- err
			}()
			close(start)
			successes := 0
			for range 2 {
				err := <-results
				if err == nil {
					successes++
				} else if !errors.Is(err, domain.ErrApplicationNotFound) && !errors.Is(err, domain.ErrApplicationNotDraft) && !errors.Is(err, domain.ErrApplicationVersionConflict) {
					t.Fatalf("unexpected race error: %v", err)
				}
			}
			if successes != 1 {
				t.Fatalf("successful actions=%d", successes)
			}
			var orphanEvents int
			if err := f.db.QueryRow(t.Context(), `SELECT count(*) FROM outbox o LEFT JOIN applications a ON a.id=o.aggregate_id WHERE a.id IS NULL`).Scan(&orphanEvents); err != nil || orphanEvents != 0 {
				t.Fatalf("orphan events=%d err=%v", orphanEvents, err)
			}
		})
	}
}
func TestDeleteDraftRollbackAndCleanupRetryIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	doc, err := f.repo.AddDocument(t.Context(), model.AddApplicationDocumentParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyKey: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "statement.txt", Content: []byte("statement")})
	if err != nil {
		t.Fatal(err)
	}
	current, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	// An unexpected reference must roll the entire deletion back, including tombstones.
	if _, err = f.db.Exec(t.Context(), `CREATE TABLE deletion_guard (id uuid REFERENCES applications(id)); INSERT INTO deletion_guard SELECT id FROM applications`); err != nil {
		t.Fatal(err)
	}
	if err = f.repo.Delete(t.Context(), deleteParams(f, current.Version)); err == nil {
		t.Fatal("expected FK failure")
	}
	if _, err = f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.db.QueryRow(t.Context(), `SELECT count(*) FROM deleted_drafts`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("tombstones=%d err=%v", count, err)
	}
	if _, err = os.Stat(filepath.Join(f.repo.storageDir, filepath.FromSlash(doc.StorageKey))); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(t.Context(), `DROP TABLE deletion_guard`); err != nil {
		t.Fatal(err)
	}
	if err = f.repo.Delete(t.Context(), deleteParams(f, current.Version)); err != nil {
		t.Fatal(err)
	}
	invalidRoot := filepath.Join(t.TempDir(), "file")
	if err = os.WriteFile(invalidRoot, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = documentcleanup.New(f.db, invalidRoot).CleanOne(t.Context()); err == nil {
		t.Fatal("expected cleanup failure")
	}
	if done, err := documentcleanup.New(f.db, f.repo.storageDir).CleanOne(t.Context()); err != nil || !done {
		t.Fatalf("retry cleanup=%v err=%v", done, err)
	}
}
