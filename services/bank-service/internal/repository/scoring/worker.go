package scoringrepository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrLeaseLost = errors.New("scoring case lease lost")
var ErrPendingAttempt = errors.New("scoring attempt is still pending")

type Claim struct {
	CaseID     uuid.UUID
	LeaseUntil time.Time
}

type CurrentAttempt struct {
	ID          *uuid.UUID
	Status      string
	Recoverable bool
}

// ClaimNext uses the cases lease so independent service instances cannot
// process the same scoring case at the same time.
func (r *Repository) ClaimNext(ctx context.Context) (Claim, bool, error) {
	var claim Claim
	err := r.db.QueryRow(ctx, claimScoringCaseQuery).Scan(&claim.CaseID, &claim.LeaseUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return Claim{}, false, nil
	}
	if err != nil {
		return Claim{}, false, fmt.Errorf("claim scoring case: %w", err)
	}
	return claim, true, nil
}

func (r *Repository) CurrentAttempt(ctx context.Context, claim Claim) (CurrentAttempt, error) {
	var current CurrentAttempt
	var status *string
	err := r.db.QueryRow(ctx, currentWorkerAttemptQuery, claim.CaseID, claim.LeaseUntil).
		Scan(&current.ID, &status, &current.Recoverable)
	if errors.Is(err, pgx.ErrNoRows) {
		return CurrentAttempt{}, ErrLeaseLost
	}
	if err != nil {
		return CurrentAttempt{}, fmt.Errorf("read current scoring attempt: %w", err)
	}
	if status != nil {
		current.Status = *status
	}
	return current, nil
}

// Finish advances only the still leased SCORING case. A completed assessment
// hands off to DECISION; all other outcomes require analyst review.
func (r *Repository) Finish(ctx context.Context, claim Claim, inputErrorCode string) (bool, error) {
	var finished bool
	err := r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		queryer := transaction.QueryerFromContext(ctx, r.db)
		var attemptID *uuid.UUID
		var attemptStatus, storedError *string
		err := queryer.QueryRow(ctx, lockWorkerCaseQuery, claim.CaseID, claim.LeaseUntil).
			Scan(&attemptID, &attemptStatus, &storedError)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("lock scoring case: %w", err)
		}
		if attemptID == nil && inputErrorCode == "" {
			return errors.New("scoring case has no attempt or input error")
		}
		if attemptID != nil && inputErrorCode != "" {
			return ErrCurrentAttempt
		}
		if attemptStatus != nil && *attemptStatus == "PENDING" {
			return ErrPendingAttempt
		}

		status, reason := scoringDestination(attemptStatus, storedError, inputErrorCode)
		var recommendation any
		if status == "MANUAL_REVIEW" {
			recommendation = "MANUAL_REVIEW"
		}
		var version int64
		err = queryer.QueryRow(ctx, finishScoringCaseQuery,
			claim.CaseID, claim.LeaseUntil, status, recommendation).Scan(&version)
		if err != nil {
			return fmt.Errorf("finish scoring case: %w", err)
		}
		details, err := json.Marshal(map[string]any{
			"attempt_id": attemptID, "assessment_status": attemptStatus,
			"to_status": status, "error_code": reason,
		})
		if err != nil {
			return fmt.Errorf("encode scoring audit: %w", err)
		}
		_, err = queryer.Exec(ctx, insertWorkerAuditQuery,
			uuid.New(), claim.CaseID, version, reason, string(details))
		if err != nil {
			return fmt.Errorf("audit scoring transition: %w", err)
		}
		finished = true
		return nil
	})
	return finished, err
}

// The legacy score is a comparison baseline. Its absence cannot replace or
// prevent the independent v2 assessment; all errors remain stored in the attempt.
func scoringDestination(status, storedError *string, inputErrorCode string) (string, string) {
	if inputErrorCode != "" {
		return "DECISION", inputErrorCode
	}
	if status == nil {
		return "DECISION", "BASELINE_MISSING_ATTEMPT"
	}
	if *status == "ASSESSED" {
		return "DECISION", ""
	}
	if storedError != nil && *storedError != "" {
		return "DECISION", *storedError
	}
	return "DECISION", "BASELINE_" + *status
}

func (r *Repository) Release(ctx context.Context, claim Claim) error {
	_, err := r.db.Exec(ctx, releaseScoringCaseQuery, claim.CaseID, claim.LeaseUntil)
	if err != nil {
		return fmt.Errorf("release scoring case: %w", err)
	}
	return nil
}
