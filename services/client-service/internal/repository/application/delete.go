package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/jackc/pgx/v5"
)

// Delete serializes with submission, autosave and document upload on the application row.
func (r *Repository) Delete(ctx context.Context, params model.DeleteApplicationParams) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin draft deletion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	var version, eventVersion int64
	err = tx.QueryRow(ctx, `SELECT status, version, client_event_version FROM applications
 WHERE id=$1 AND org_id=$2 FOR UPDATE`, params.ApplicationID, params.OrgID).Scan(&status, &version, &eventVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrApplicationNotFound
	}
	if err != nil {
		return fmt.Errorf("lock draft for deletion: %w", err)
	}
	if status != "DRAFT" || eventVersion != 0 {
		return domain.ErrApplicationNotDraft
	}
	if version != params.ExpectedVersion {
		return domain.ErrApplicationVersionConflict
	}
	if _, err = tx.Exec(ctx, `INSERT INTO deleted_drafts(application_id, org_id) VALUES ($1,$2)`, params.ApplicationID, params.OrgID); err != nil {
		return fmt.Errorf("record draft deletion: %w", err)
	}
	// Keep creation keys as tombstones so a delayed retry cannot recreate the draft.
	if _, err = tx.Exec(ctx, `DELETE FROM idempotency_keys WHERE application_id=$1 AND route<>$2`, params.ApplicationID, idempotencyRoute); err != nil {
		return fmt.Errorf("delete draft action keys: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE idempotency_keys SET application_id=NULL, response_status=410, response_body='{}'::jsonb, response_headers='{}'::jsonb WHERE application_id=$1`, params.ApplicationID); err != nil {
		return fmt.Errorf("tombstone draft creation keys: %w", err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM application_documents WHERE application_id=$1`, params.ApplicationID); err != nil {
		return fmt.Errorf("delete draft document metadata: %w", err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM timeline_entries WHERE application_id=$1`, params.ApplicationID); err != nil {
		return fmt.Errorf("delete draft timeline: %w", err)
	}
	// Unexpected bank state or outbox references deliberately prevent deletion via FKs.
	if _, err = tx.Exec(ctx, `DELETE FROM applications WHERE id=$1 AND org_id=$2`, params.ApplicationID, params.OrgID); err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit draft deletion: %w", err)
	}
	return nil
}
