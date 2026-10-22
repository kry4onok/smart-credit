package scoring

import (
	"context"
	"errors"
	"testing"
	"time"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type rpcStub struct {
	calls int
	score func(context.Context, *scoringpb.ScoreRequest) (*scoringpb.ScoreResponse, error)
}

func (stub *rpcStub) Score(ctx context.Context, request *scoringpb.ScoreRequest, _ ...grpc.CallOption) (*scoringpb.ScoreResponse, error) {
	stub.calls++
	return stub.score(ctx, request)
}

func TestScoreRetriesUnavailableWithinOneDeadline(t *testing.T) {
	stub := &rpcStub{}
	stub.score = func(ctx context.Context, request *scoringpb.ScoreRequest) (*scoringpb.ScoreResponse, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 2*time.Second {
			t.Fatal("scoring RPC has no two-second deadline")
		}
		if request.Features.TotalDebt == nil || request.Features.CompanyAgeMonths == nil || *request.Features.CompanyAgeMonths != 36 {
			t.Fatalf("scoring request = %+v", request)
		}
		if stub.calls == 1 {
			return nil, status.Error(codes.Unavailable, "temporary outage")
		}
		return validAssessedResponse(), nil
	}

	response, err := New(stub).Score(t.Context(), validInput())
	if err != nil || response.GetScore() != 938 || stub.calls != 2 {
		t.Fatalf("Score() response=%v err=%v calls=%d", response, err, stub.calls)
	}
}

func TestScoreDoesNotRetrySchemaError(t *testing.T) {
	stub := &rpcStub{score: func(context.Context, *scoringpb.ScoreRequest) (*scoringpb.ScoreResponse, error) {
		return nil, status.Error(codes.FailedPrecondition, "unknown feature version")
	}}
	_, err := New(stub).Score(t.Context(), validInput())
	if status.Code(errors.Unwrap(err)) != codes.FailedPrecondition || stub.calls != 1 {
		t.Fatalf("Score() err=%v calls=%d", err, stub.calls)
	}
}

func TestValidateResponseDistinguishesZeroFromMissingScore(t *testing.T) {
	response := validAssessedResponse()
	zero := int32(0)
	response.Score = &zero
	response.Grade = scoringpb.Grade_GRADE_E
	response.FactorContributions[0].Points = -888
	if err := ValidateResponse(response); err != nil {
		t.Fatalf("valid score 0 rejected: %v", err)
	}

	response.Score = nil
	if !errors.Is(ValidateResponse(response), ErrInvalidResponse) {
		t.Fatal("missing score accepted as score 0")
	}

	response = &scoringpb.ScoreResponse{
		AssessmentStatus:     scoringpb.AssessmentStatus_ASSESSMENT_STATUS_INSUFFICIENT_DATA,
		Grade:                scoringpb.Grade_GRADE_UNSPECIFIED,
		DataIssues:           []*scoringpb.DataIssue{{Code: "MISSING_VALUE", FieldPath: "features.total_debt"}},
		ModelVersion:         ModelVersion,
		FeatureSchemaVersion: FeatureSchemaVersion,
		BaselineScore:        500,
		ConfigChecksum:       checksumForTest,
	}
	if err := ValidateResponse(response); err != nil {
		t.Fatalf("valid missing-data response rejected: %v", err)
	}
}

const checksumForTest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func validAssessedResponse() *scoringpb.ScoreResponse {
	score := int32(938)
	return &scoringpb.ScoreResponse{
		Score:                &score,
		Grade:                scoringpb.Grade_GRADE_A,
		ModelVersion:         ModelVersion,
		FeatureSchemaVersion: FeatureSchemaVersion,
		AssessmentStatus:     scoringpb.AssessmentStatus_ASSESSMENT_STATUS_ASSESSED,
		BaselineScore:        500,
		ConfigChecksum:       checksumForTest,
		FactorContributions: []*scoringpb.FactorContribution{
			{FactorName: "business_age", Points: 50, Description: "age"},
			{FactorName: "profit_margin", Points: 100, Description: "profit"},
			{FactorName: "bureau_debt_load", Points: 88, Description: "debt"},
			{FactorName: "payment_discipline", Points: 200, Description: "overdue"},
		},
	}
}
