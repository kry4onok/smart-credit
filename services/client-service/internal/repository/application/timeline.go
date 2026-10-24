package application

import (
	"context"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func (r *Repository) Timeline(ctx context.Context, orgID, applicationID uuid.UUID) ([]model.TimelineEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.status, t.message, t.occurred_at
		FROM applications a
		LEFT JOIN timeline_entries t ON t.application_id = a.id
		WHERE a.id = $1 AND a.org_id = $2
		ORDER BY t.occurred_at, t.id`, applicationID, orgID)
	if err != nil {
		return nil, fmt.Errorf("fetch application timeline: %w", err)
	}
	defer rows.Close()

	entries := make([]model.TimelineEntry, 0)
	found := false
	for rows.Next() {
		found = true
		var id *uuid.UUID
		var status, message *string
		var occurredAt *time.Time
		if err := rows.Scan(&id, &status, &message, &occurredAt); err != nil {
			return nil, fmt.Errorf("scan application timeline: %w", err)
		}
		if id == nil {
			continue
		}
		entries = append(entries, model.TimelineEntry{ID: *id, Status: *status, Message: *message, OccurredAt: *occurredAt})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate application timeline: %w", err)
	}
	if !found {
		return nil, domain.ErrApplicationNotFound
	}
	return entries, nil
}
