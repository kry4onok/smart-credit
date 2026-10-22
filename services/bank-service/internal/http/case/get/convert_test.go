package gethandler

import (
	"encoding/json"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
)

func TestToScoreKeepsZeroAsAssessed(t *testing.T) {
	result := toScore(model.ScoreResult{
		Score:                0,
		Grade:                model.ScoreGrade("E"),
		ModelVersion:         "monotone-scorecard-v1",
		FeatureSchemaVersion: "borrower-features-v1",
	})

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal score: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatalf("unmarshal score: %v", err)
	}
	if body["assessment_status"] != "ASSESSED" || body["score"] != float64(0) {
		t.Fatalf("score response = %s, want ASSESSED with present score 0", encoded)
	}
	if _, ok := body["data_issues"].([]any); !ok {
		t.Fatalf("score response = %s, want data_issues array", encoded)
	}
}
