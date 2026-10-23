package scoring

import (
	"context"
	"errors"
	"fmt"
	"time"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	scoringclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/scoring"
	scoringrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/scoring"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Attempts interface {
	LoadInput(context.Context, uuid.UUID) (scoringrepository.SavedInput, error)
	Start(context.Context, scoringrepository.Attempt) error
	Complete(context.Context, uuid.UUID, *scoringpb.ScoreResponse, string) (bool, error)
}

type Scorer interface {
	Score(context.Context, scoringclient.Input) (*scoringpb.ScoreResponse, error)
}

var ErrInvalidInput = errors.New("scoring input cannot be built")

type Runner struct {
	attempts Attempts
	scorer   Scorer
	now      func() time.Time
}

func New(attempts Attempts, scorer Scorer) *Runner {
	return &Runner{attempts: attempts, scorer: scorer, now: time.Now}
}

func (r *Runner) Run(ctx context.Context, caseID uuid.UUID) (uuid.UUID, bool, error) {
	saved, err := r.attempts.LoadInput(ctx, caseID)
	if err != nil {
		return uuid.Nil, false, err
	}
	saved.Input.AsOfDate = r.now().UTC().Truncate(24 * time.Hour)
	request, err := scoringclient.BuildRequest(saved.Input)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	attemptID := uuid.New()
	if err := r.attempts.Start(ctx, scoringrepository.Attempt{
		ID: attemptID, CaseID: caseID, InputRevision: saved.Revision,
		AsOfDate: saved.Input.AsOfDate, Request: request,
	}); err != nil {
		return uuid.Nil, false, err
	}

	response, scoreErr := r.scorer.Score(ctx, saved.Input)
	errorCode := ""
	if scoreErr != nil {
		response = nil
		errorCode = classifyError(scoreErr)
	}
	// Record the outcome even if the caller's context expired during gRPC.
	completeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	applied, completeErr := r.attempts.Complete(completeCtx, attemptID, response, errorCode)
	if completeErr != nil {
		return attemptID, false, fmt.Errorf("complete scoring attempt: %w", completeErr)
	}
	if scoreErr != nil {
		return attemptID, applied, fmt.Errorf("scoring attempt %s: %w", attemptID, scoreErr)
	}
	return attemptID, applied, nil
}

func classifyError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded), status.Code(err) == codes.DeadlineExceeded:
		return "SCORING_DEADLINE"
	case status.Code(err) == codes.Unavailable:
		return "SCORING_UNAVAILABLE"
	case errors.Is(err, scoringclient.ErrInvalidResponse):
		return "SCORING_INVALID_RESPONSE"
	default:
		return "SCORING_ERROR"
	}
}
