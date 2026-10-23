package scoring

import (
	"context"
	"errors"
	"testing"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	scoringclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/scoring"
	scoringrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/scoring"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type attemptStub struct {
	saved     scoringrepository.SavedInput
	started   *scoringrepository.Attempt
	completed bool
	code      string
}

func (s *attemptStub) LoadInput(context.Context, uuid.UUID) (scoringrepository.SavedInput, error) {
	return s.saved, nil
}
func (s *attemptStub) Start(_ context.Context, attempt scoringrepository.Attempt) error {
	s.started = &attempt
	return nil
}
func (s *attemptStub) Complete(_ context.Context, id uuid.UUID, _ *scoringpb.ScoreResponse, code string) (bool, error) {
	if s.started == nil || id != s.started.ID {
		return false, errors.New("scoring was completed before it started")
	}
	s.completed, s.code = true, code
	return true, nil
}

type scorerStub struct {
	attempts *attemptStub
	err      error
}

func (s scorerStub) Score(_ context.Context, input scoringclient.Input) (*scoringpb.ScoreResponse, error) {
	if s.attempts.started == nil || !input.AsOfDate.Equal(s.attempts.started.AsOfDate) {
		return nil, errors.New("scoring called before attempt with frozen date")
	}
	if s.err != nil {
		return nil, s.err
	}
	return &scoringpb.ScoreResponse{}, nil
}

func TestRunPersistsAttemptAndTechnicalOutcome(t *testing.T) {
	caseID := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"domain response", nil, ""},
		{"unavailable", status.Error(codes.Unavailable, "down"), "SCORING_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := &attemptStub{saved: scoringrepository.SavedInput{
				Revision: 3,
				Input: scoringclient.Input{
					ApplicationID: caseID,
					ProductID:     uuid.MustParse("11111111-1111-4111-8111-111111111111"),
					Company:       &commonpb.CompanySnapshot{Revenue_12M: &commonpb.Money{AmountKopecks: 1, Currency: "RUB"}},
					Profile:       &bureaupb.CompanyProfileResponse{RegisteredAt: "2020-01-01"},
					CreditHistory: &bureaupb.CreditHistoryResponse{TotalDebt: &commonpb.Money{Currency: "RUB"}},
				},
			}}
			runner := New(attempts, scorerStub{attempts: attempts, err: tc.err})
			runner.now = func() time.Time { return time.Date(2026, 10, 23, 19, 0, 0, 0, time.UTC) }
			_, applied, err := runner.Run(t.Context(), caseID)
			if applied != true || (err != nil) != (tc.err != nil) || !attempts.completed || attempts.code != tc.code {
				t.Fatalf("applied=%t error=%v completed=%t code=%q", applied, err, attempts.completed, attempts.code)
			}
			if attempts.started.InputRevision != 3 || attempts.started.AsOfDate.Hour() != 0 {
				t.Fatalf("attempt not frozen: %+v", attempts.started)
			}
		})
	}
}
