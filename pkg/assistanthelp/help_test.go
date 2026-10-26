package assistanthelp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	pb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/assistant"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestPythonGoParity(t *testing.T) {
	data, err := os.ReadFile("testdata/parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		ID            string
		Now           time.Time
		Request       json.RawMessage
		Operation     string
		Reason        string
		Plan          json.RawMessage
		Expected      json.RawMessage
		ExpectedError string   `json:"expected_error"`
		IDs           []string `json:"expected_statement_ids"`
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 30 {
		t.Fatalf("want 30 full fixtures, got %d", len(fixtures))
	}
	for _, f := range fixtures {
		t.Run(f.ID, func(t *testing.T) {
			var request pb.HelpRequest
			if err = protojson.Unmarshal(f.Request, &request); err != nil {
				t.Fatal(err)
			}
			if f.ExpectedError != "" {
				if err := Validate(&request, f.Now); err == nil || err.Error() != f.ExpectedError {
					t.Fatalf("validation=%v want %s", err, f.ExpectedError)
				}
				return
			}
			var got *pb.HelpResponse
			var e error
			if f.Operation == "render" {
				var plan struct {
					Status string
					Blocks []struct {
						Kind       string
						Selections []struct {
							StatementID string `json:"statement_id"`
							VariantID   string `json:"variant_id"`
						}
					}
				}
				if e = json.Unmarshal(f.Plan, &plan); e != nil {
					t.Fatal(e)
				}
				p := &pb.AnswerPlan{Status: pb.AnswerStatus(pb.AnswerStatus_value["ANSWER_STATUS_"+plan.Status])}
				for _, b := range plan.Blocks {
					block := &pb.PlanBlock{Kind: pb.BlockKind(pb.BlockKind_value["BLOCK_KIND_"+b.Kind])}
					for _, s := range b.Selections {
						block.Selections = append(block.Selections, &pb.Selection{StatementId: s.StatementID, VariantId: s.VariantID})
					}
					p.Blocks = append(p.Blocks, block)
				}
				got, e = Render(&request, p, pb.ResponseSource_RESPONSE_SOURCE_MODEL, "fixture-model", pb.FallbackReason_FALLBACK_REASON_NONE, f.Now)
			} else {
				got, e = Fallback(&request, pb.FallbackReason(pb.FallbackReason_value["FALLBACK_REASON_"+f.Reason]), f.Now)
			}
			if e != nil {
				t.Fatal(e)
			}
			var expected pb.HelpResponse
			if e = protojson.Unmarshal(f.Expected, &expected); e != nil {
				t.Fatal(e)
			}
			if !proto.Equal(got, &expected) {
				t.Fatalf("Go/Python mismatch:\nGo %s\nPython %s", protojson.Format(got), protojson.Format(&expected))
			}
			ids := []string{}
			for _, b := range got.Blocks {
				for _, s := range b.Items {
					if s.StatementId != "" {
						ids = append(ids, s.StatementId)
					}
				}
			}
			if !reflect.DeepEqual(ids, f.IDs) {
				t.Fatalf("unexpected approved statements: %v", ids)
			}
		})
	}
}

func TestHelpRejectsStalePostInferenceAndIncompatibleCatalog(t *testing.T) {
	now := time.Now().UTC()
	v := View{ApplicationID: "22222222-2222-4222-8222-222222222222", Revision: "generation:1", Status: "MANUAL_REVIEW", NextStep: "Откройте заявку"}
	c := Context(v, pb.Audience_AUDIENCE_CLIENT, now)
	body, _ := json.Marshal(SelectionRequest{ContextID: c.ContextId, SourceRevision: c.SourceRevision, CatalogVersion: CatalogVersion, Mode: "NEXT_STEP", Detail: "CONCISE"})
	calls := 0
	load := func(context.Context) (View, error) {
		calls++
		out := v
		if calls == 2 {
			out.Revision = "generation:2"
		}
		return out, nil
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/help", strings.NewReader(string(body)))
	ServeHTTP(w, r, "", pb.Audience_AUDIENCE_CLIENT, load)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "CONTEXT_STALE") {
		t.Fatalf("stale result delivered: %d %s", w.Code, w.Body)
	}
	body = []byte(strings.ReplaceAll(string(body), CatalogVersion, "unknown-catalog"))
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/help", strings.NewReader(string(body)))
	ServeHTTP(w, r, "", pb.Audience_AUDIENCE_CLIENT, func(context.Context) (View, error) { return v, nil })
	if w.Code != 409 || !strings.Contains(w.Body.String(), "UNSUPPORTED_CATALOG_VERSION") {
		t.Fatalf("unknown catalogue fallback: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/help", nil)
	ServeHTTP(w, r, "", pb.Audience_AUDIENCE_CLIENT, func(context.Context) (View, error) { return View{}, errors.New("not owned") })
	if w.Code != 404 {
		t.Fatalf("unauthorized source status=%d", w.Code)
	}
}

func TestRendererNeverAcceptsUnknownStatementOrCompanionCycle(t *testing.T) {
	now := time.Now().UTC()
	c := Context(View{ApplicationID: "22222222-2222-4222-8222-222222222222", Revision: "1", Status: "DRAFT"}, pb.Audience_AUDIENCE_CLIENT, now)
	r := &pb.HelpRequest{RequestId: "11111111-1111-4111-8111-111111111111", Context: c, Mode: pb.HelpMode_HELP_MODE_NEXT_STEP, Detail: pb.Detail_DETAIL_CONCISE, Locale: "ru-RU"}
	p := &pb.AnswerPlan{Status: pb.AnswerStatus_ANSWER_STATUS_ANSWERED, Blocks: []*pb.PlanBlock{{Kind: pb.BlockKind_BLOCK_KIND_DETAILS, Selections: []*pb.Selection{{StatementId: "invented", VariantId: "concise"}}}}}
	if _, err := Render(r, p, pb.ResponseSource_RESPONSE_SOURCE_MODEL, "evil", pb.FallbackReason_FALLBACK_REASON_NONE, now); err == nil {
		t.Fatal("accepted invented statement")
	}
	c.Statements[0].CompanionIds = []string{c.Statements[0].Id}
	if err := Validate(r, now); err == nil {
		t.Fatal("accepted companion cycle")
	}
}
