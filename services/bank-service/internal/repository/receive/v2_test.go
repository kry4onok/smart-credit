package receiverepository

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
)

func TestMergeDocumentSnapshotsIsCumulativeAndImmutable(t *testing.T) {
	period := &commonpb.DatePeriod{From: "2025-10-01", To: "2026-09-30"}
	old := receive.DocumentSnapshot{DocumentID: "old", DocumentType: "BANK_STATEMENT", SHA256: "first", UploadedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), RequirementID: "cash_flow:2025-10-01:2026-09-30:all", CoveragePeriod: period}
	newDoc := receive.DocumentSnapshot{DocumentID: "new", DocumentType: "MANAGEMENT_REPORT", SHA256: "second", UploadedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), RequirementID: "business_result:2025-10-01:2026-09-30:all", CoveragePeriod: period}
	stored, _ := json.Marshal([]receive.DocumentSnapshot{old})
	incoming, _ := json.Marshal([]receive.DocumentSnapshot{newDoc})
	merged, pb, conflict, err := mergeDocumentSnapshots(stored, incoming)
	if err != nil || conflict || len(pb) != 2 {
		t.Fatalf("merge: %v, conflict=%v, documents=%d", err, conflict, len(pb))
	}
	var got []receive.DocumentSnapshot
	if err := json.Unmarshal(merged, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []receive.DocumentSnapshot{old, newDoc}) {
		t.Fatalf("merged snapshot: %+v", got)
	}
	if pb[0].GetDocumentId() != "old" || pb[1].GetRequirementId() != newDoc.RequirementID {
		t.Fatalf("protobuf documents: %v", pb)
	}
	changed := old
	changed.SHA256 = "different"
	bad, _ := json.Marshal([]receive.DocumentSnapshot{changed})
	_, _, conflict, err = mergeDocumentSnapshots(stored, bad)
	if err != nil || !conflict {
		t.Fatalf("changed same ID: conflict=%v, error=%v", conflict, err)
	}
}

func TestV2RequestRoundTripAndChangedMatrix(t *testing.T) {
	field := &commonpb.Requirement{Id: "company.profit_12m", Kind: commonpb.RequirementKind_REQUIREMENT_KIND_FIELD, Status: commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED, FieldPath: "company.profit_12m"}
	doc := &commonpb.Requirement{Id: "cash_flow:2025-10-01:2026-09-30:all", Kind: commonpb.RequirementKind_REQUIREMENT_KIND_DOCUMENT, Status: commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING, Period: &commonpb.DatePeriod{From: "2025-10-01", To: "2026-09-30"}}
	data, err := encodeRequirements([]*commonpb.Requirement{field, doc})
	if err != nil {
		t.Fatal(err)
	}
	request, err := decodeRequirements(data)
	if err != nil || len(request) != 2 || request[1].Id != doc.Id {
		t.Fatalf("decoded request: %v, %v", request, err)
	}
	plan := &commonpb.RequirementsPlan{Items: []*commonpb.Requirement{field, doc}}
	missing := unmetRequest(request, plan, nil)
	if len(missing) != 2 {
		t.Fatalf("missing without explicit field answer: %v", missing)
	}
	missing = unmetRequest(request, plan, []string{field.Id})
	if len(missing) != 1 || missing[0].Id != doc.Id {
		t.Fatalf("missing after field answer: %v", missing)
	}
	plan.Items = []*commonpb.Requirement{field} // A valid profile correction removed the old document rule.
	if missing = unmetRequest(request, plan, []string{field.Id}); len(missing) != 0 {
		t.Fatalf("obsolete requirement remained: %v", missing)
	}
}
