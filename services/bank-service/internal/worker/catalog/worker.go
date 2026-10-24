package catalogworker

import (
	"context"
	"log/slog"
	"time"
)

const pollInterval = 5 * time.Second
const operationTimeout = 15 * time.Second

type Store interface {
	RunOnce(context.Context) (bool, error)
}

type Worker struct {
	store Store
}

func New(store Store) *Worker {
	return &Worker{store: store}
}

func (w *Worker) Run(ctx context.Context, logger *slog.Logger) {
	for ctx.Err() == nil {
		operationCtx, cancel := context.WithTimeout(ctx, operationTimeout)
		changed, err := w.store.RunOnce(operationCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Error("bank catalog publication failed", "error", err)
		} else if changed {
			logger.Info("bank product catalog queued for publication")
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
