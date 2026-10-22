package scoring

import (
	"os"
	"testing"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	"github.com/google/uuid"
)

// TestLiveScoringAdapter runs only when a scoring endpoint is explicitly provided.
func TestLiveScoringAdapter(t *testing.T) {
	target := os.Getenv("BANK_SCORING_TARGET")
	if target == "" {
		t.Skip("set BANK_SCORING_TARGET to run the live scoring adapter check")
	}

	client, err := Dial(target)
	if err != nil {
		t.Fatalf("connect to scoring: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close scoring connection: %v", err)
		}
	})

	cases := []struct {
		name       string
		caseID     string
		change     func(*Input)
		status     scoringpb.AssessmentStatus
		score      int32
		grade      scoringpb.Grade
		issueCount int
	}{
		{
			name: "risk_a", caseID: "c1000000-0000-4000-8000-000000000001",
			status: scoringpb.AssessmentStatus_ASSESSMENT_STATUS_ASSESSED,
			score:  938, grade: scoringpb.Grade_GRADE_A,
		},
		{
			name: "zero_debt", caseID: "c1000000-0000-4000-8000-000000000006",
			change: func(input *Input) { input.CreditHistory.TotalDebt.AmountKopecks = 0 },
			status: scoringpb.AssessmentStatus_ASSESSMENT_STATUS_ASSESSED,
			score:  975, grade: scoringpb.Grade_GRADE_A,
		},
		{
			name: "score_zero", caseID: "c1000000-0000-4000-8000-000000000007",
			change: func(input *Input) {
				input.Profile.RegisteredAt = "2026-10-01"
				input.Company.Profit_12M.AmountKopecks = -120_000_000
				input.CreditHistory.TotalDebt.AmountKopecks = 2_400_000_000
				count, days := int32(6), int32(7)
				input.CreditHistory.OverdueCount_12M = &count
				input.CreditHistory.MaxOverdueDays = &days
			},
			status: scoringpb.AssessmentStatus_ASSESSMENT_STATUS_ASSESSED,
			score:  0, grade: scoringpb.Grade_GRADE_E,
		},
		{
			name: "missing_debt", caseID: "c1000000-0000-4000-8000-000000000009",
			change:     func(input *Input) { input.CreditHistory.TotalDebt = nil },
			status:     scoringpb.AssessmentStatus_ASSESSMENT_STATUS_INSUFFICIENT_DATA,
			issueCount: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := validInput()
			input.ApplicationID = uuid.MustParse(tc.caseID)
			if tc.change != nil {
				tc.change(&input)
			}
			response, err := client.Score(t.Context(), input)
			if err != nil {
				t.Fatalf("adapter Score(): %v", err)
			}
			if response.AssessmentStatus != tc.status || len(response.DataIssues) != tc.issueCount {
				t.Fatalf("status=%s issues=%v; want status=%s issue count=%d", response.AssessmentStatus, response.DataIssues, tc.status, tc.issueCount)
			}
			if tc.status == scoringpb.AssessmentStatus_ASSESSMENT_STATUS_ASSESSED {
				if response.Score == nil || *response.Score != tc.score || response.Grade != tc.grade {
					t.Fatalf("score=%v grade=%s; want score=%d grade=%s", response.Score, response.Grade, tc.score, tc.grade)
				}
				t.Logf("status=%s score=%d grade=%s", response.AssessmentStatus, *response.Score, response.Grade)
			} else if response.Score != nil {
				t.Fatalf("unassessed response has score %d", *response.Score)
			} else {
				if len(response.DataIssues) != 1 || response.DataIssues[0].Code != "MISSING_VALUE" || response.DataIssues[0].FieldPath != "features.total_debt" {
					t.Fatalf("data issues=%v; want missing total debt", response.DataIssues)
				}
				t.Logf("status=%s score=absent issue=%s:%s", response.AssessmentStatus, response.DataIssues[0].Code, response.DataIssues[0].FieldPath)
			}
		})
	}
}
