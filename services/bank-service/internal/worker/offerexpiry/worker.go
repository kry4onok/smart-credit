package offerexpiryworker

import (
	"context"
	"log/slog"
	"time"
)

const pollInterval = time.Second
const operationTimeout = 5 * time.Second

type Runner interface {
	RunOnce(context.Context) (bool, error)
}

type Worker struct {
	runner Runner
}

func New(runner Runner) *Worker {
	return &Worker{runner: runner}
}

func (w *Worker) Run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		operationCtx, cancel := context.WithTimeout(ctx, operationTimeout)
		_, err := w.runner.RunOnce(operationCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Error("offer expiry worker failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
