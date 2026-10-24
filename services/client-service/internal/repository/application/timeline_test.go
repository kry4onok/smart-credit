package application

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func TestTimelineIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	initial, err := f.repo.Timeline(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || len(initial) != 1 || initial[0].Status != "DRAFT" || initial[0].Message != "Черновик создан" {
		t.Fatalf("initial timeline=%+v, err=%v", initial, err)
	}
	firstID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	secondID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	lastID := uuid.New()
	tiedTime := initial[0].OccurredAt.Add(time.Minute)
	lastTime := tiedTime.Add(time.Minute)
	// Insert out of chronological order and reverse UUID order for equal times.
	for _, entry := range []model.TimelineEntry{
		{ID: lastID, Status: "UNDER_REVIEW", Message: "На рассмотрении", OccurredAt: lastTime},
		{ID: secondID, Status: "SUBMITTED", Message: "Второе действие", OccurredAt: tiedTime},
		{ID: firstID, Status: "SUBMITTED", Message: "Первое действие", OccurredAt: tiedTime},
	} {
		if _, err := f.db.Exec(t.Context(), `
			INSERT INTO timeline_entries (id, application_id, status, action, actor_type, message, occurred_at)
			VALUES ($1, $2, $3, 'BANK_STATE', 'SYSTEM', $4, $5)`,
			entry.ID, f.draft.ID, entry.Status, entry.Message, entry.OccurredAt); err != nil {
			t.Fatal(err)
		}
	}
	otherParams := f.params
	otherParams.IdempotencyID = uuid.New()
	other, err := f.repo.Create(t.Context(), otherParams)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.repo.Timeline(t.Context(), f.params.OrgID, f.draft.ID)
	want := append(initial,
		model.TimelineEntry{ID: firstID, Status: "SUBMITTED", Message: "Первое действие", OccurredAt: tiedTime},
		model.TimelineEntry{ID: secondID, Status: "SUBMITTED", Message: "Второе действие", OccurredAt: tiedTime},
		model.TimelineEntry{ID: lastID, Status: "UNDER_REVIEW", Message: "На рассмотрении", OccurredAt: lastTime},
	)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("timeline=%+v, want=%+v, err=%v", got, want, err)
	}
	for _, target := range []struct {
		name string
		org  uuid.UUID
		id   uuid.UUID
	}{
		{"foreign application", uuid.New(), f.draft.ID},
		{"missing application", f.params.OrgID, uuid.New()},
		{"zero ID", f.params.OrgID, uuid.Nil},
	} {
		t.Run(target.name, func(t *testing.T) {
			if _, err := f.repo.Timeline(t.Context(), target.org, target.id); !errors.Is(err, domain.ErrApplicationNotFound) {
				t.Fatalf("err=%v, want application not found", err)
			}
		})
	}
	if _, err := f.db.Exec(t.Context(), "DELETE FROM timeline_entries WHERE application_id = $1", other.ID); err != nil {
		t.Fatal(err)
	}
	empty, err := f.repo.Timeline(t.Context(), f.params.OrgID, other.ID)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty application history=%+v, err=%v", empty, err)
	}
}
