package scoringworker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	scoringrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/scoring"
	stage "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/scoring"
	"github.com/google/uuid"
)

const pollInterval = 2 * time.Second
const operationTimeout = 15 * time.Second

type Queue interface {
	ClaimNext(context.Context) (scoringrepository.Claim, bool, error)
	CurrentAttempt(context.Context, scoringrepository.Claim) (scoringrepository.CurrentAttempt, error)
	Finish(context.Context, scoringrepository.Claim, string) (bool, error)
	Release(context.Context, scoringrepository.Claim) error
}

type Runner interface {
	Run(context.Context, uuid.UUID) (uuid.UUID, bool, error)
}

type Completer interface {
	Complete(context.Context, uuid.UUID, *scoringpb.ScoreResponse, string) (bool, error)
}

type Worker struct {
	queue     Queue
	runner    Runner
	completer Completer
}

func New(queue Queue, runner Runner, completer Completer) *Worker {
	return &Worker{queue: queue, runner: runner, completer: completer}
}

func (w *Worker) Run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if _, err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			logger.Error("scoring worker failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) RunOnce(parent context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, operationTimeout)
	defer cancel()
	claim, found, err := w.queue.ClaimNext(ctx)
	if err != nil || !found {
		return found, err
	}
	if err := w.process(ctx, claim); err != nil {
		releaseCtx, releaseCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer releaseCancel()
		return true, errors.Join(err, w.queue.Release(releaseCtx, claim))
	}
	return true, nil
}

func (w *Worker) process(ctx context.Context, claim scoringrepository.Claim) error {
	current, err := w.queue.CurrentAttempt(ctx, claim)
	if err != nil {
		return err
	}
	var scoreErr error
	if current.ID == nil {
		_, _, scoreErr = w.runner.Run(ctx, claim.CaseID)
		if code := inputErrorCode(scoreErr); code != "" {
			_, err = w.queue.Finish(ctx, claim, code)
			return err
		}
		current, err = w.queue.CurrentAttempt(ctx, claim)
		if err != nil {
			return errors.Join(scoreErr, err)
		}
		if current.ID == nil {
			return errors.Join(scoreErr, errors.New("scoring run left no attempt"))
		}
	}
	if current.Status == "PENDING" {
		if !current.Recoverable {
			return scoringrepository.ErrPendingAttempt
		}
		_, err = w.completer.Complete(ctx, *current.ID, nil, "SCORING_INTERRUPTED")
		if err != nil && !errors.Is(err, scoringrepository.ErrAttemptCompleted) {
			return errors.Join(scoreErr, err)
		}
	}
	_, err = w.queue.Finish(ctx, claim, "")
	return errors.Join(scoreErr, err)
}

func inputErrorCode(err error) string {
	switch {
	case errors.Is(err, scoringrepository.ErrInputNotReady):
		return "SCORING_INPUT_NOT_READY"
	case errors.Is(err, scoringrepository.ErrMalformedInput), errors.Is(err, stage.ErrInvalidInput):
		return "SCORING_INVALID_INPUT"
	default:
		return ""
	}
}
