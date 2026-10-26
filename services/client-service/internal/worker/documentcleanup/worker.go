// Package documentcleanup removes files only after a draft deletion commits.
package documentcleanup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	db         *pgxpool.Pool
	storageDir string
}

func New(db *pgxpool.Pool, storageDir string) *Worker { return &Worker{db: db, storageDir: storageDir} }
func (w *Worker) Run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		cleaned, err := w.CleanOne(ctx)
		if err != nil {
			logger.Error("draft document cleanup failed", "error", err)
		}
		if err == nil && cleaned {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// CleanOne is retry-safe even if removing files succeeds but the DB commit fails.
func (w *Worker) CleanOne(ctx context.Context) (bool, error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id uuid.UUID
	err = tx.QueryRow(ctx, `SELECT application_id FROM deleted_drafts WHERE documents_cleaned_at IS NULL ORDER BY deleted_at,application_id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	root, err := os.OpenRoot(w.storageDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("open document root: %w", err)
	}
	if err == nil {
		defer func() { _ = root.Close() }()
		if err = root.RemoveAll(id.String()); err != nil {
			return false, fmt.Errorf("remove deleted draft documents: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE deleted_drafts SET documents_cleaned_at=now() WHERE application_id=$1`, id); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
