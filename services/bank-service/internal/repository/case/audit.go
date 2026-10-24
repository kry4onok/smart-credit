package caserepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	auditcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const auditCaseExistsQuery = `
SELECT 1 
FROM cases 
WHERE id=$1`

const listCaseAuditQuery = `
SELECT id,action,actor_type,actor_id,details->>'from_status',details->>'to_status',reason,occurred_at
FROM audit_log
WHERE case_id=$1 AND ($2::timestamptz IS NULL OR (occurred_at,id)>($2::timestamptz,$3::uuid))
ORDER BY occurred_at,id
LIMIT $4`

func (r *Repository) CaseExists(ctx context.Context, caseID uuid.UUID) (bool, error) {
	var one int
	err := transaction.QueryerFromContext(ctx, r.db).QueryRow(ctx, auditCaseExistsQuery, caseID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check case existence: %w", err)
	}
	return true, nil
}

func (r *Repository) ListAudit(ctx context.Context, query auditcase.Query) ([]auditcase.Entry, error) {
	var afterTime *time.Time
	var afterID *uuid.UUID
	if query.After != nil {
		afterTime = &query.After.OccurredAt
		afterID = &query.After.ID
	}
	rows, err := transaction.QueryerFromContext(ctx, r.db).Query(ctx, listCaseAuditQuery, query.CaseID, afterTime, afterID, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	defer rows.Close()
	items := make([]auditcase.Entry, 0, query.Limit)
	for rows.Next() {
		var item auditcase.Entry
		if err = rows.Scan(&item.ID, &item.Action, &item.ActorType, &item.ActorID, &item.FromStatus, &item.ToStatus, &item.Reason, &item.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit log: %w", err)
	}
	return items, nil
}

var _ auditcase.Repository = (*Repository)(nil)
