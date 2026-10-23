package outboxworker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	outboxrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/outbox"
)

const pollInterval = 2 * time.Second
const operationTimeout = 15 * time.Second
const finishTimeout = 5 * time.Second

var ErrLeaseLost = errors.New("outbox event lease lost")

type Store interface {
	ClaimNext(context.Context) (outboxrepository.Event, bool, error)
	MarkPublished(context.Context, outboxrepository.Event) (bool, error)
	Release(context.Context, outboxrepository.Event) error
}

type Publisher interface {
	Publish(context.Context, outboxrepository.Event) error
}

type Worker struct {
	store     Store
	publisher Publisher
}

func New(store Store, publisher Publisher) *Worker {
	return &Worker{store: store, publisher: publisher}
}

func (w *Worker) Run(ctx context.Context, logger *slog.Logger) {
	for ctx.Err() == nil {
		processed, err := w.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			logger.Error("bank outbox relay failed", "error", err)
		}
		if processed && err == nil {
			continue
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

func (w *Worker) RunOnce(parent context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, operationTimeout)
	defer cancel()
	event, found, err := w.store.ClaimNext(ctx)
	if err != nil || !found {
		return found, err
	}
	if err := w.publisher.Publish(ctx, event); err != nil {
		return true, errors.Join(
			fmt.Errorf("publish outbox event %s: %w", event.ID, err),
			w.release(parent, event),
		)
	}
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(parent), finishTimeout)
	defer finishCancel()
	marked, err := w.store.MarkPublished(finishCtx, event)
	if err != nil {
		return true, errors.Join(
			fmt.Errorf("finish outbox event %s: %w", event.ID, err),
			w.release(parent, event),
		)
	}
	if !marked {
		return true, fmt.Errorf("outbox event %s: %w", event.ID, ErrLeaseLost)
	}
	return true, nil
}

func (w *Worker) release(parent context.Context, event outboxrepository.Event) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), finishTimeout)
	defer cancel()
	return w.store.Release(ctx, event)
}
