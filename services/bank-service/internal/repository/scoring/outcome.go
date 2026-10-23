package scoringrepository

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	scoringclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/scoring"
	"google.golang.org/protobuf/encoding/protojson"
)

type preparedOutcome struct {
	status       string
	responseJSON []byte
	reasonsJSON  []byte
	errorCode    string
	response     *scoringpb.ScoreResponse
}

func prepareOutcome(response *scoringpb.ScoreResponse, errorCode string) (preparedOutcome, error) {
	if (response == nil) == (errorCode == "") {
		return preparedOutcome{}, errors.New("provide either a scoring response or an error code")
	}
	if response == nil {
		return preparedOutcome{status: "TECHNICAL_ERROR", errorCode: errorCode}, nil
	}
	if err := scoringclient.ValidateResponse(response); err != nil {
		return preparedOutcome{}, err
	}

	encoded, err := protojson.Marshal(response)
	if err != nil {
		return preparedOutcome{}, fmt.Errorf("encode scoring response: %w", err)
	}

	outcome := preparedOutcome{
		status:       strings.TrimPrefix(response.AssessmentStatus.String(), "ASSESSMENT_STATUS_"),
		responseJSON: encoded,
		response:     response,
	}

	if outcome.status == "ASSESSED" {
		outcome.reasonsJSON, err = encodeReasons(response.Reasons)
		if err != nil {
			return preparedOutcome{}, err
		}
	}

	return outcome, nil
}

func (o preparedOutcome) isDomainResult() bool { return o.status != "TECHNICAL_ERROR" }
func (o preparedOutcome) isAssessed() bool     { return o.status == "ASSESSED" }

func nullableJSON(value []byte) any {
	if value == nil {
		return nil
	}

	return string(value)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func gradeName(grade scoringpb.Grade) string {
	return strings.TrimPrefix(grade.String(), "GRADE_")
}

func encodeReasons(reasons []*scoringpb.Reason) ([]byte, error) {
	type reasonRecord struct {
		Code         string `json:"code"`
		Description  string `json:"description"`
		Direction    string `json:"direction"`
		Contribution int32  `json:"contribution"`
	}

	records := make([]reasonRecord, 0, len(reasons))
	for _, reason := range reasons {
		records = append(records, reasonRecord{
			Code:         strings.TrimPrefix(reason.Code.String(), "REASON_CODE_"),
			Description:  reason.Description,
			Direction:    strings.TrimPrefix(reason.Direction.String(), "DIRECTION_"),
			Contribution: reason.Contribution,
		})
	}

	result, err := json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("encode scoring reasons: %w", err)
	}

	return result, nil
}
