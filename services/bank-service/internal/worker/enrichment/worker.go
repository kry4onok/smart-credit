package enrichmentworker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	bureauclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/bureau"
	enrichmentrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/enrichment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const pollInterval = 2 * time.Second
const operationTimeout = 15 * time.Second

type Queue interface {
	ClaimNext(context.Context) (enrichmentrepository.Claim, bool, error)
	Complete(context.Context, enrichmentrepository.Claim, enrichmentrepository.Outcome) (bool, error)
	Release(context.Context, enrichmentrepository.Claim) error
}

type Bureau interface {
	GetCompanyProfile(context.Context, string) (*bureaupb.CompanyProfileResponse, error)
	GetCreditHistory(context.Context, string) (*bureaupb.CreditHistoryResponse, error)
}

type Worker struct {
	queue  Queue
	bureau Bureau
}

func New(queue Queue, bureau Bureau) *Worker {
	return &Worker{queue: queue, bureau: bureau}
}

func (w *Worker) Run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if _, err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			logger.Error("enrichment worker failed", "error", err)
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
	profile, profileErr := w.bureau.GetCompanyProfile(ctx, claim.INN)
	credit, creditErr := w.bureau.GetCreditHistory(ctx, claim.INN)
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finishCancel()
	if parent.Err() != nil {
		return true, errors.Join(parent.Err(), w.queue.Release(finishCtx, claim))
	}
	_, err = w.queue.Complete(finishCtx, claim, enrichmentrepository.Outcome{
		Profile: profile, ProfileError: errorCode(profileErr),
		Credit: credit, CreditError: errorCode(creditErr),
	})
	if err != nil {
		return true, errors.Join(err, w.queue.Release(finishCtx, claim))
	}
	return true, nil
}

func errorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded), status.Code(err) == codes.DeadlineExceeded:
		return "BUREAU_TIMEOUT"
	case status.Code(err) == codes.Unavailable:
		return "BUREAU_UNAVAILABLE"
	case status.Code(err) == codes.NotFound:
		return "BUREAU_NOT_FOUND"
	case status.Code(err) == codes.InvalidArgument, errors.Is(err, bureauclient.ErrInvalidResponse):
		return "BUREAU_INVALID_RESPONSE"
	default:
		return "BUREAU_ERROR"
	}
}
