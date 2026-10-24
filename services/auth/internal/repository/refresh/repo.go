package refresh

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db transaction.Executor }

func New(db transaction.Executor) *Repository { return &Repository{db: db} }

func (r *Repository) Save(ctx context.Context, session model.RefreshSession) error {
	_, err := transaction.ExecutorFor(ctx, r.db).Exec(ctx, `
	INSERT INTO refresh_tokens (token_hash, user_id, created_at, expires_at) VALUES ($1, $2, $3, $4)`,
		session.Hash[:], session.UserID, session.CreatedAt, session.ExpiresAt)
	if err != nil {
		return fmt.Errorf("save refresh session: %w", err)
	}
	return nil
}

// Lock serializes token issuance with logout and user changes. Check the expiry
// in the usecase after acquiring the lock, rather than before a possible wait.
func (r *Repository) Lock(ctx context.Context, hash [32]byte) (model.SessionState, error) {
	var session model.SessionState
	var orgID *uuid.UUID
	err := transaction.ExecutorFor(ctx, r.db).QueryRow(ctx, `
	SELECT u.id, u.org_id, u.role, u.can_submit, u.is_active, t.created_at, t.expires_at, t.revoked_at
	FROM refresh_tokens t JOIN users u ON u.id = t.user_id
	WHERE t.token_hash = $1 FOR SHARE OF t, u`, hash[:]).Scan(
		&session.UserID, &orgID, &session.Role, &session.CanSubmit, &session.Active,
		&session.CreatedAt, &session.ExpiresAt, &session.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SessionState{}, domain.ErrUnauthorized
	}
	if err != nil {
		return model.SessionState{}, fmt.Errorf("fetch refresh session: %w", err)
	}
	if orgID != nil {
		session.OrgID = *orgID
	}
	return session, nil
}

func (r *Repository) Revoke(ctx context.Context, hash [32]byte) error {
	_, err := transaction.ExecutorFor(ctx, r.db).Exec(ctx, `
	UPDATE refresh_tokens SET revoked_at = greatest(clock_timestamp(), created_at)
	WHERE token_hash = $1 AND revoked_at IS NULL`, hash[:])
	if err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	return nil
}
