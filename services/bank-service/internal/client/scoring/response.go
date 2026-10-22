package scoring

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
)

const ModelVersion = "monotone-scorecard-v1"

var ErrInvalidResponse = errors.New("invalid scoring response")

func ValidateResponse(response *scoringpb.ScoreResponse) error {
	if response == nil {
		return invalidResponse("missing response")
	}
	if response.ModelVersion != ModelVersion || response.FeatureSchemaVersion != FeatureSchemaVersion ||
		response.BaselineScore != 500 || !validChecksum(response.ConfigChecksum) {
		return invalidResponse("invalid model metadata")
	}
	if err := validateDataIssues(response.DataIssues); err != nil {
		return err
	}

	switch response.AssessmentStatus {
	case scoringpb.AssessmentStatus_ASSESSMENT_STATUS_ASSESSED:
		if response.Score == nil || *response.Score < 0 || *response.Score > 1000 ||
			response.Grade != gradeForScore(*response.Score) || len(response.DataIssues) != 0 {
			return invalidResponse("invalid assessed score, grade, or data issues")
		}
		if err := validateFactors(response); err != nil {
			return err
		}
		return validateReasons(response.Reasons)
	case scoringpb.AssessmentStatus_ASSESSMENT_STATUS_INSUFFICIENT_DATA,
		scoringpb.AssessmentStatus_ASSESSMENT_STATUS_UNSUPPORTED_INPUT:
		if response.Score != nil || response.Grade != scoringpb.Grade_GRADE_UNSPECIFIED ||
			len(response.Reasons) != 0 || len(response.FactorContributions) != 0 || len(response.DataIssues) == 0 {
			return invalidResponse("invalid unassessed result")
		}
		return nil
	default:
		return invalidResponse("unspecified assessment status")
	}
}

func validChecksum(value string) bool {
	if len(value) != 64 {
		return false
	}

	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func gradeForScore(score int32) scoringpb.Grade {
	switch {
	case score >= 800:
		return scoringpb.Grade_GRADE_A
	case score >= 650:
		return scoringpb.Grade_GRADE_B
	case score >= 500:
		return scoringpb.Grade_GRADE_C
	case score >= 350:
		return scoringpb.Grade_GRADE_D
	default:
		return scoringpb.Grade_GRADE_E
	}
}

func validateFactors(response *scoringpb.ScoreResponse) error {
	if len(response.FactorContributions) != 4 {
		return invalidResponse("expected four factor contributions")
	}
	want := map[string]bool{
		"business_age":       false,
		"profit_margin":      false,
		"bureau_debt_load":   false,
		"payment_discipline": false,
	}
	sum := int64(response.BaselineScore)
	for _, factor := range response.FactorContributions {
		if factor == nil || factor.Description == "" {
			return invalidResponse("invalid factor contribution")
		}
		seen, exists := want[factor.FactorName]
		if !exists || seen {
			return invalidResponse("unknown or duplicate factor contribution")
		}
		want[factor.FactorName] = true
		sum += int64(factor.Points)
	}
	if sum != int64(*response.Score) {
		return invalidResponse("factor contributions do not sum to score")
	}
	return nil
}

func validateReasons(reasons []*scoringpb.Reason) error {
	if len(reasons) > 3 {
		return invalidResponse("too many reasons")
	}
	var previous *scoringpb.Reason
	for _, reason := range reasons {
		if reason == nil || reason.Description == "" || reason.Contribution <= 0 ||
			reason.Code == scoringpb.ReasonCode_REASON_CODE_UNSPECIFIED ||
			reason.Direction == scoringpb.Direction_DIRECTION_UNSPECIFIED {
			return invalidResponse("invalid reason")
		}
		if _, known := scoringpb.ReasonCode_name[int32(reason.Code)]; !known {
			return invalidResponse("unknown reason code")
		}
		if _, known := scoringpb.Direction_name[int32(reason.Direction)]; !known {
			return invalidResponse("unknown reason direction")
		}
		if previous != nil && (previous.Contribution < reason.Contribution ||
			(previous.Contribution == reason.Contribution && reasonName(previous) > reasonName(reason))) {
			return invalidResponse("reasons are out of order")
		}
		previous = reason
	}
	return nil
}

func reasonName(reason *scoringpb.Reason) string {
	return strings.TrimPrefix(reason.Code.String(), "REASON_CODE_")
}

func validateDataIssues(issues []*scoringpb.DataIssue) error {
	previous := ""
	for _, issue := range issues {
		if issue == nil || issue.FieldPath == "" {
			return invalidResponse("invalid data issue")
		}
		switch issue.Code {
		case "MISSING_VALUE", "ZERO_REVENUE", "INCONSISTENT_OVERDUE":
		default:
			return invalidResponse("unknown data issue code")
		}
		key := issue.FieldPath + "\x00" + issue.Code
		if key < previous {
			return invalidResponse("data issues are out of order")
		}
		previous = key
	}
	return nil
}

func invalidResponse(detail string) error {
	return fmt.Errorf("%w: %s", ErrInvalidResponse, detail)
}
