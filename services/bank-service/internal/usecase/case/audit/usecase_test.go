package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repoStub struct {
	exists bool
	items  []Entry
	query  Query
	calls  int
}

func (r *repoStub) CaseExists(context.Context, uuid.UUID) (bool, error) { return r.exists, nil }
func (r *repoStub) ListAudit(_ context.Context, q Query) ([]Entry, error) {
	r.query = q
	r.calls++
	return r.items, nil
}
func TestAuditPaginatesSameTimestampAndBindsCursorToCase(t *testing.T) {
	caseID := uuid.New()
	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	first, second, third := Entry{ID: uuid.New(), OccurredAt: at}, Entry{ID: uuid.New(), OccurredAt: at}, Entry{ID: uuid.New(), OccurredAt: at.Add(time.Second)}
	repo := &repoStub{exists: true, items: []Entry{first, second, third}}
	limit := 2
	out, err := New(repo).Execute(t.Context(), Input{CaseID: caseID, Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.NextCursor == nil || out.Items[1].ID != second.ID || repo.query.Limit != 3 {
		t.Fatalf("page=%+v query=%+v", out, repo.query)
	}
	decoded, err := decodeCursor(caseID, out.NextCursor)
	if err != nil || decoded.ID != second.ID || !decoded.OccurredAt.Equal(at) {
		t.Fatalf("cursor=%+v error=%v", decoded, err)
	}
	_, err = New(repo).Execute(t.Context(), Input{CaseID: uuid.New(), Cursor: out.NextCursor})
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("foreign case cursor=%v", err)
	}
}
func TestAuditDistinguishesMissingCaseAndEmptyHistory(t *testing.T) {
	repo := &repoStub{}
	_, err := New(repo).Execute(t.Context(), Input{CaseID: uuid.New()})
	if !errors.Is(err, ErrNotFound) || repo.calls != 0 {
		t.Fatalf("missing case=%v calls=%d", err, repo.calls)
	}
	repo.exists = true
	repo.items = []Entry{}
	out, err := New(repo).Execute(t.Context(), Input{CaseID: uuid.New()})
	if err != nil || out.Items == nil || len(out.Items) != 0 || out.NextCursor != nil {
		t.Fatalf("empty history=%+v error=%v", out, err)
	}
}
func TestAuditRejectsInvalidLimitAndCursor(t *testing.T) {
	repo := &repoStub{exists: true}
	zero := 0
	if _, err := New(repo).Execute(t.Context(), Input{CaseID: uuid.New(), Limit: &zero}); !errors.Is(err, ErrInvalidLimit) {
		t.Fatal(err)
	}
	malformed := "not base64!"
	if _, err := New(repo).Execute(t.Context(), Input{CaseID: uuid.New(), Cursor: &malformed}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatal(err)
	}
}
