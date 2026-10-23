package scoringrepository

import "testing"

func TestScoringDestination(t *testing.T) {
	for _, tc := range []struct {
		status     string
		errorCode  string
		wantStatus string
		wantReason string
	}{
		{"ASSESSED", "", "DECISION", ""},
		{"INSUFFICIENT_DATA", "", "MANUAL_REVIEW", "SCORING_INSUFFICIENT_DATA"},
		{"UNSUPPORTED_INPUT", "", "MANUAL_REVIEW", "SCORING_UNSUPPORTED_INPUT"},
		{"TECHNICAL_ERROR", "SCORING_UNAVAILABLE", "MANUAL_REVIEW", "SCORING_UNAVAILABLE"},
	} {
		t.Run(tc.status+tc.errorCode, func(t *testing.T) {
			status, errorCode := tc.status, tc.errorCode
			gotStatus, gotReason := scoringDestination(&status, &errorCode, "")
			if gotStatus != tc.wantStatus || gotReason != tc.wantReason {
				t.Fatalf("destination=(%q,%q); want (%q,%q)", gotStatus, gotReason, tc.wantStatus, tc.wantReason)
			}
		})
	}
}
