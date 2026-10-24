package user

import (
	"context"
	"errors"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/postgres"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) FetchCurrent(ctx context.Context, id uuid.UUID) (model.CurrentUser, error) {
	var user model.CurrentUser
	var orgID *uuid.UUID
	var inn, name *string
	err := transaction.ExecutorFor(ctx, r.db).QueryRow(ctx, `
	SELECT u.id, u.email, u.full_name, u.role, u.can_submit, u.is_active, o.id, o.inn, o.name
	FROM users u LEFT JOIN organizations o ON o.id = u.org_id WHERE u.id = $1`, id).Scan(
		&user.ID, &user.Email, &user.FullName, &user.Role, &user.CanSubmit, &user.Active, &orgID, &inn, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.CurrentUser{}, domain.ErrUserNotFound
	}
	if err != nil {
		return model.CurrentUser{}, postgres.OperationError("fetch current auth user", err)
	}
	if orgID != nil && inn != nil && name != nil {
		user.Organization = &model.Organization{ID: *orgID, INN: *inn, Name: *name}
	}
	return user, nil
}
