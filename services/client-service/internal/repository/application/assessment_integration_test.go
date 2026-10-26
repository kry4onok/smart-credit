package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/consumer/bankevents"
	assessmenthandler "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/assessment"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type assessmentPrincipal struct{ p model.Principal }

func (v assessmentPrincipal) Verify(context.Context, string) (model.Principal, error) {
	return v.p, nil
}

func TestAssessmentProjectionOrderingPrivacyAndHTTPIsolation(t *testing.T) {
	f := newApplicationFixture(t)
	f.submit(t)
	state := &eventspb.ApplicationPublicStatePublished{Status: eventspb.ClientStatus_CLIENT_STATUS_UNDER_REVIEW, StageLabel: "Synthetic assessment"}
	// Even an accidentally over-broad bank payload must not leak private fields.
	state.AssessmentJson = []byte(`{"schema_version":1,"assessment_id":"current","status":"MANUAL_REVIEW","risk":{"score":123},"checks":{"sources":[{"subject_id":"private"}]}}`)
	old := f.project(t, 2, state)
	read := func() string {
		t.Helper()
		var b []byte
		if err := f.db.QueryRow(t.Context(), `SELECT public_assessment FROM applications WHERE id=$1`, f.draft.ID).Scan(&b); err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := read(); strings.Contains(got, "risk") || strings.Contains(got, "private") || !strings.Contains(got, "current") {
		t.Fatal(got)
	}
	state.AssessmentJson = nil
	f.project(t, 3, state)
	if got := read(); got != "" {
		t.Fatalf("new generation retained old assessment: %s", got)
	}
	if applied, err := bankevents.NewProjection(f.db).Execute(t.Context(), old); err != nil || applied {
		t.Fatalf("replay: %v %v", applied, err)
	}
	if got := read(); got != "" {
		t.Fatal("replay resurrected assessment")
	}
	// New HTTP routes all enforce the organization in the SQL predicate.
	for _, foreign := range []bool{false, true} {
		org := f.params.OrgID
		if foreign {
			org = uuid.New()
		}
		principal := assessmentPrincipal{model.Principal{OrgID: org, UserID: uuid.New(), Role: "CLIENT"}}
		h := middleware.RequireClient(principal, assessmenthandler.New(f.db, "").Wrap(http.NotFoundHandler()))
		for _, path := range []string{"assessment", "assessment-input", "help"} {
			req := httptest.NewRequest("GET", "/client/applications/"+f.draft.ID.String()+"/"+path, nil)
			req.Header.Set("Authorization", "Bearer synthetic")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			want := 200
			if foreign {
				want = 404
			}
			if w.Code != want {
				t.Fatalf("%s foreign=%v: %d %s", path, foreign, w.Code, w.Body.String())
			}
		}
	}
	result := assessment.PublicResult{SchemaVersion: 1, ID: "next", Generation: 2, ValidUntil: time.Now().Add(time.Hour)}
	state.AssessmentJson, _ = json.Marshal(result)
	f.project(t, 4, state)
	// Status-only full snapshots carry the same public result.
	state.Status = eventspb.ClientStatus_CLIENT_STATUS_DISBURSED
	f.project(t, 5, state)
	if got := read(); !strings.Contains(got, "next") {
		t.Fatal("status transition lost assessment")
	}
}
