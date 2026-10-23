package caserepository

import (
	"testing"
	"time"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestDecodeCurrentScorePreservesZeroAndAttemptMetadata(t *testing.T) {
	zero := int32(0)
	response := &scoringpb.ScoreResponse{
		AssessmentStatus:     scoringpb.AssessmentStatus_ASSESSMENT_STATUS_ASSESSED,
		Score:                &zero,
		Grade:                scoringpb.Grade_GRADE_E,
		ModelVersion:         "monotone-scorecard-v1",
		FeatureSchemaVersion: "borrower-features-v1",
		BaselineScore:        500,
		ConfigChecksum:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Reasons: []*scoringpb.Reason{{
			Code:        scoringpb.ReasonCode_REASON_CODE_RECENT_OVERDUE,
			Direction:   scoringpb.Direction_DIRECTION_INCREASES_RISK,
			Description: "recent overdue", Contribution: 200,
		}},
		FactorContributions: []*scoringpb.FactorContribution{{
			FactorName: "business_age", Points: -50, Description: "age",
		}},
	}
	encoded, err := protojson.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	attemptID := uuid.New()
	asOf := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	result, err := decodeCurrentScore(encoded, attemptID, 4, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if result.Score == nil || *result.Score != 0 || result.AttemptID == nil || *result.AttemptID != attemptID ||
		result.InputRevision == nil || *result.InputRevision != 4 || !result.AsOfDate.Equal(asOf) {
		t.Fatalf("score or attempt metadata lost: %+v", result)
	}
	if len(result.Reasons) != 1 || result.Reasons[0].Code != "RECENT_OVERDUE" ||
		result.Reasons[0].Direction != "INCREASES_RISK" || len(result.FactorContributions) != 1 {
		t.Fatalf("score details lost: %+v", result)
	}
}

func TestDecodeCurrentScoreKeepsUnassessedScoreAbsent(t *testing.T) {
	response := &scoringpb.ScoreResponse{
		AssessmentStatus:     scoringpb.AssessmentStatus_ASSESSMENT_STATUS_INSUFFICIENT_DATA,
		Grade:                scoringpb.Grade_GRADE_UNSPECIFIED,
		ModelVersion:         "monotone-scorecard-v1",
		FeatureSchemaVersion: "borrower-features-v1",
		DataIssues: []*scoringpb.DataIssue{{
			Code: "MISSING_VALUE", FieldPath: "features.total_debt",
		}},
	}

	encoded, err := protojson.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	result, err := decodeCurrentScore(encoded, uuid.New(), 2, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.AssessmentStatus != "INSUFFICIENT_DATA" || result.Score != nil ||
		result.Grade != "UNSPECIFIED" || len(result.DataIssues) != 1 ||
		result.DataIssues[0].FieldPath != "features.total_debt" {
		t.Fatalf("unassessed result = %+v", result)
	}
}
