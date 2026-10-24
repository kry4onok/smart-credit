// Package devseed creates fixed local demo identities outside auth migrations.
package devseed

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/password"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	organizationID  = "20000000-0000-4000-8000-000000000001"
	clientID        = "30000000-0000-4000-8000-000000000001"
	analystID       = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	adminID         = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	organizationINN = "7707083893"

	insertOrganizationQuery = `INSERT INTO organizations (id, inn, name)
		VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`
	selectOrganizationQuery = `SELECT inn FROM organizations WHERE id = $1`
	insertUserQuery         = `INSERT INTO users (id, email, full_name, password_hash, role, org_id, can_submit)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING`
	selectUserQuery = `SELECT email, role, org_id, can_submit, is_active, password_hash
		FROM users WHERE id = $1`
	updatePasswordQuery = `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`
	revokeSessionsQuery = `DELETE FROM refresh_tokens WHERE user_id = $1`
)

type Credentials struct {
	AnalystPassword string
	AdminPassword   string
	ClientPassword  string
	ResetPasswords  bool
}

type Account struct {
	ID    uuid.UUID
	Email string
	Role  string
}

type identity struct {
	Account
	name      string
	orgID     uuid.UUID
	canSubmit bool
	secret    string
}

func (c Credentials) Validate() error {
	for _, item := range []struct{ name, value string }{
		{"AUTH_DEV_ANALYST_PASSWORD", c.AnalystPassword},
		{"AUTH_DEV_ADMIN_PASSWORD", c.AdminPassword},
		{"AUTH_DEV_CLIENT_PASSWORD", c.ClientPassword},
	} {
		if strings.TrimSpace(item.value) == "" {
			return fmt.Errorf("%s must be set to a nonempty password", item.name)
		}
	}
	return nil
}

func Run(ctx context.Context, db *pgxpool.Pool, credentials Credentials) ([]Account, error) {
	if err := credentials.Validate(); err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("auth database is required")
	}
	orgID := uuid.MustParse(organizationID)
	identities := []identity{
		{Account: Account{ID: uuid.MustParse(analystID), Email: "analyst@smart-credit.test", Role: "ANALYST"},
			name: "Демо аналитик", secret: credentials.AnalystPassword},
		{Account: Account{ID: uuid.MustParse(adminID), Email: "admin@smart-credit.test", Role: "ADMIN"},
			name: "Демо администратор", secret: credentials.AdminPassword},
		{Account: Account{ID: uuid.MustParse(clientID), Email: "client@smart-credit.test", Role: "CLIENT"},
			name: "Демо клиент", orgID: orgID, canSubmit: true, secret: credentials.ClientPassword},
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("start auth dev seed: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureOrganization(ctx, tx, orgID); err != nil {
		return nil, err
	}
	accounts := make([]Account, 0, len(identities))
	for _, person := range identities {
		if err := ensureUser(ctx, tx, person, credentials.ResetPasswords); err != nil {
			return nil, err
		}
		accounts = append(accounts, person.Account)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit auth dev seed: %w", err)
	}
	return accounts, nil
}

func ensureOrganization(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	if _, err := tx.Exec(ctx, insertOrganizationQuery, id, organizationINN, "ООО «Северная пекарня»"); err != nil {
		return fmt.Errorf("insert demo organization: %w", err)
	}
	var inn string
	if err := tx.QueryRow(ctx, selectOrganizationQuery, id).Scan(&inn); err != nil {
		return fmt.Errorf("read demo organization: %w", err)
	}
	if inn != organizationINN {
		return errors.New("demo organization ID already belongs to another INN")
	}
	return nil
}

func ensureUser(ctx context.Context, tx pgx.Tx, person identity, resetPassword bool) error {
	var org any
	if person.orgID != uuid.Nil {
		org = person.orgID
	}
	hasher := password.New()
	var existing struct {
		email     string
		role      string
		orgID     *uuid.UUID
		canSubmit bool
		active    bool
		hash      string
	}
	err := tx.QueryRow(ctx, selectUserQuery, person.ID).Scan(
		&existing.email, &existing.role, &existing.orgID, &existing.canSubmit, &existing.active, &existing.hash)
	if errors.Is(err, pgx.ErrNoRows) {
		hash, hashErr := hasher.Hash(person.secret)
		if hashErr != nil {
			return fmt.Errorf("hash demo %s password: %w", person.Role, hashErr)
		}
		if _, err := tx.Exec(ctx, insertUserQuery, person.ID, person.Email, person.name, hash,
			person.Role, org, person.canSubmit); err != nil {
			return fmt.Errorf("insert demo %s: %w", person.Role, err)
		}
		err = tx.QueryRow(ctx, selectUserQuery, person.ID).Scan(
			&existing.email, &existing.role, &existing.orgID, &existing.canSubmit, &existing.active, &existing.hash)
	}
	if err != nil {
		return fmt.Errorf("read demo %s: %w", person.Role, err)
	}
	if existing.email != person.Email || existing.role != person.Role || !existing.active ||
		existing.canSubmit != person.canSubmit || !sameOrg(existing.orgID, person.orgID) {
		return fmt.Errorf("demo %s ID already belongs to a different account", person.Role)
	}
	match, err := hasher.Verify(existing.hash, person.secret)
	if err != nil || !match {
		if !resetPassword {
			return fmt.Errorf("demo %s password differs from existing account", person.Role)
		}
		hash, hashErr := hasher.Hash(person.secret)
		if hashErr != nil {
			return fmt.Errorf("hash new demo %s password: %w", person.Role, hashErr)
		}
		if _, err := tx.Exec(ctx, updatePasswordQuery, person.ID, hash); err != nil {
			return fmt.Errorf("update demo %s password: %w", person.Role, err)
		}
		if _, err := tx.Exec(ctx, revokeSessionsQuery, person.ID); err != nil {
			return fmt.Errorf("revoke demo %s sessions: %w", person.Role, err)
		}
	}
	return nil
}

func sameOrg(actual *uuid.UUID, expected uuid.UUID) bool {
	if expected == uuid.Nil {
		return actual == nil
	}
	return actual != nil && *actual == expected
}
