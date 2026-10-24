package user

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

func (r *Repository) FindByEmail(ctx context.Context, email string) (model.User, error) {
	return scanUser(transaction.ExecutorFor(ctx, r.db).QueryRow(ctx, `
	SELECT id, org_id, role, can_submit, password_hash, is_active FROM users WHERE lower(email) = lower($1)`, email))
}

// LockByID keeps credentials and permissions stable until login commits.
func (r *Repository) LockByID(ctx context.Context, id uuid.UUID) (model.User, error) {
	return scanUser(transaction.ExecutorFor(ctx, r.db).QueryRow(ctx, `
	SELECT id, org_id, role, can_submit, password_hash, is_active FROM users WHERE id = $1 FOR SHARE`, id))
}

func scanUser(row pgx.Row) (model.User, error) {
	var user model.User
	var orgID *uuid.UUID
	err := row.Scan(&user.UserID, &orgID, &user.Role, &user.CanSubmit, &user.PasswordHash, &user.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("fetch auth user: %w", err)
	}
	if orgID != nil {
		user.OrgID = *orgID
	}
	return user, nil
}
