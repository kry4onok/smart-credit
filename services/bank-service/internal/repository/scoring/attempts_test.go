package scoringrepository

import (
	"encoding/json"
	"testing"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
)

func TestEncodeReasonsMatchesBankCardContract(t *testing.T) {
	data, err := encodeReasons([]*scoringpb.Reason{{
		Code:         scoringpb.ReasonCode_REASON_CODE_NO_OVERDUE,
		Description:  "no overdue",
		Direction:    scoringpb.Direction_DIRECTION_DECREASES_RISK,
		Contribution: 200,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0]["code"] != "NO_OVERDUE" ||
		records[0]["direction"] != "DECREASES_RISK" {
		t.Fatalf("stored reasons = %s", data)
	}
}
