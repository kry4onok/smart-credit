package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	topic        = "client.events"
	pollInterval = 500 * time.Millisecond
	leasePeriod  = 30 * time.Second
)

type DB interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type record struct {
	eventID          uuid.UUID
	aggregateID      uuid.UUID
	aggregateVersion int64
	payload          []byte
}

type Worker struct {
	db     DB
	client *kgo.Client
}

func New(db DB, brokers []string) (*Worker, error) {
	if db == nil || len(brokers) == 0 {
		return nil, errors.New("outbox database and Kafka brokers are required")
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return nil, fmt.Errorf("create outbox Kafka producer: %w", err)
	}
	return &Worker{db: db, client: client}, nil
}

func (w *Worker) Close() {
	if w != nil && w.client != nil {
		w.client.Close()
	}
}

func (w *Worker) Run(ctx context.Context, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	for ctx.Err() == nil {
		published, err := w.publishOne(ctx)
		if err != nil {
			logger.Error("client outbox relay failed", "topic", topic, "error", err)
			if !wait(ctx, pollInterval) {
				return
			}
			continue
		}
		if published {
			continue
		}
		if !wait(ctx, pollInterval) {
			return
		}
	}
}

func (w *Worker) publishOne(ctx context.Context) (bool, error) {
	var item record
	err := w.db.QueryRow(ctx, `
		WITH candidate AS (
			SELECT o.event_id
			FROM outbox o
			WHERE o.published_at IS NULL
			  AND (o.lease_until IS NULL OR o.lease_until < now())
			  AND NOT EXISTS (
				  SELECT 1
				  FROM outbox previous
				  WHERE previous.aggregate_id = o.aggregate_id
				    AND previous.published_at IS NULL
				    AND previous.aggregate_version < o.aggregate_version
			  )
			ORDER BY o.created_at, o.event_id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox o
		SET lease_until = now() + $1::interval, attempts = o.attempts + 1
		FROM candidate
		WHERE o.event_id = candidate.event_id
		RETURNING o.event_id, o.aggregate_id, o.aggregate_version, o.payload`,
		leasePeriod.String()).Scan(&item.eventID, &item.aggregateID, &item.aggregateVersion, &item.payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim outbox event: %w", err)
	}

	result := w.client.ProduceSync(ctx, &kgo.Record{
		Topic: topic, Key: []byte(item.aggregateID.String()), Value: item.payload,
	})
	if err := result.FirstErr(); err != nil {
		return false, fmt.Errorf("publish event %s: %w", item.eventID, err)
	}
	if _, err := w.db.Exec(ctx, `
		UPDATE outbox
		SET published_at = now(), lease_until = NULL
		WHERE event_id = $1 AND published_at IS NULL`, item.eventID); err != nil {
		return false, fmt.Errorf("mark outbox event %s published: %w", item.eventID, err)
	}
	return true, nil
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
