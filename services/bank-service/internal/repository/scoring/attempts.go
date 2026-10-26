package scoringrepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	scoringclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/scoring"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	ErrCaseNotFound     = errors.New("scoring case not found")
	ErrStaleInput       = errors.New("scoring input revision changed")
	ErrAttemptNotFound  = errors.New("scoring attempt not found")
	ErrAttemptCompleted = errors.New("scoring attempt already completed")
	ErrCurrentAttempt   = errors.New("current scoring attempt already exists")
)

type Attempt struct {
	ID            uuid.UUID
	CaseID        uuid.UUID
	InputRevision int64
	Generation    int64
	AsOfDate      time.Time
	Request       *scoringpb.ScoreRequest
}

type Repository struct {
	db           transaction.Queryer
	transactions *transaction.Manager
}

func New(db transaction.Queryer, transactions *transaction.Manager) *Repository {
	return &Repository{db: db, transactions: transactions}
}

func (r *Repository) Start(ctx context.Context, attempt Attempt) error {
	if err := validateAttempt(attempt); err != nil {
		return err
	}
	requestJSON, err := protojson.Marshal(attempt.Request)
	if err != nil {
		return fmt.Errorf("encode scoring request: %w", err)
	}

	return r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		queryer := transaction.QueryerFromContext(ctx, r.db)
		if err := lockCurrentRevision(ctx, queryer, attempt.CaseID, attempt.InputRevision, attempt.Generation); err != nil {
			return err
		}

		_, err := queryer.Exec(ctx, insertAttemptQuery,
			attempt.ID, attempt.CaseID, attempt.InputRevision,
			attempt.AsOfDate.UTC().Format(time.DateOnly), string(requestJSON), max(int64(1), attempt.Generation))
		if err != nil {
			return fmt.Errorf("insert scoring attempt: %w", err)
		}
		_, err = queryer.Exec(ctx, selectCurrentAttemptQuery, attempt.ID, attempt.CaseID)
		if err != nil {
			return fmt.Errorf("select current scoring attempt: %w", err)
		}
		return nil
	})
}

func validateAttempt(attempt Attempt) error {
	if attempt.ID == uuid.Nil || attempt.CaseID == uuid.Nil || attempt.InputRevision < 1 ||
		attempt.AsOfDate.IsZero() || attempt.Request == nil ||
		attempt.Request.GetApplicationId() != attempt.CaseID.String() ||
		attempt.Request.GetFeatureSchemaVersion() != scoringclient.FeatureSchemaVersion ||
		attempt.Request.GetFeatures() == nil {
		return errors.New("invalid scoring attempt")
	}
	return nil
}

func lockCurrentRevision(ctx context.Context, queryer transaction.Queryer, caseID uuid.UUID, expected int64, expectedGeneration int64) error {
	var revision, generation int64
	var status *string
	err := queryer.QueryRow(ctx, lockCaseRevisionQuery, caseID).Scan(&revision, &status, &generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCaseNotFound
	}
	if err != nil {
		return fmt.Errorf("lock case for scoring: %w", err)
	}
	if revision != expected || generation != max(int64(1), expectedGeneration) {
		return ErrStaleInput
	}
	if status != nil && *status != "TECHNICAL_ERROR" {
		return ErrCurrentAttempt
	}
	return nil
}

func (r *Repository) Invalidate(ctx context.Context, caseID uuid.UUID, expectedRevision int64) (int64, error) {
	if caseID == uuid.Nil || expectedRevision < 1 {
		return 0, errors.New("invalid scoring input revision")
	}
	var revision int64
	err := transaction.QueryerFromContext(ctx, r.db).
		QueryRow(ctx, invalidateInputQuery, caseID, expectedRevision).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrStaleInput
	}
	if err != nil {
		return 0, fmt.Errorf("invalidate scoring input: %w", err)
	}
	return revision, nil
}

func (r *Repository) Complete(
	ctx context.Context, attemptID uuid.UUID, response *scoringpb.ScoreResponse, errorCode string,
) (bool, error) {
	if attemptID == uuid.Nil {
		return false, errors.New("invalid scoring attempt ID")
	}
	outcome, err := prepareOutcome(response, errorCode)
	if err != nil {
		return false, err
	}

	var applied bool
	err = r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		queryer := transaction.QueryerFromContext(ctx, r.db)
		attempt, err := lockPendingAttempt(ctx, queryer, attemptID)
		if err != nil {
			return err
		}

		_, err = queryer.Exec(ctx, completeAttemptQuery,
			attemptID, outcome.status, nullableJSON(outcome.responseJSON), nullableString(outcome.errorCode))
		if err != nil {
			return fmt.Errorf("complete scoring attempt: %w", err)
		}

		err = queryer.QueryRow(ctx, currentAttemptQuery,
			attemptID, attempt.revision, attempt.caseID).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check current scoring attempt: %w", err)
		}
		if !applied || !outcome.isDomainResult() {
			return nil
		}
		_, err = queryer.Exec(ctx, advanceCaseVersionQuery, attempt.caseID)
		if err != nil {
			return fmt.Errorf("advance case version for scoring result: %w", err)
		}
		if !outcome.isAssessed() {
			return nil
		}

		_, err = queryer.Exec(ctx, insertAssessedScoreQuery,
			attemptID, *outcome.response.Score, gradeName(outcome.response.Grade),
			string(outcome.reasonsJSON), outcome.response.ModelVersion,
			outcome.response.FeatureSchemaVersion)
		if err != nil {
			return fmt.Errorf("insert assessed score: %w", err)
		}
		return nil
	})
	return applied, err
}

type pendingAttempt struct {
	caseID   uuid.UUID
	revision int64
}

func lockPendingAttempt(ctx context.Context, queryer transaction.Queryer, attemptID uuid.UUID) (pendingAttempt, error) {
	var attempt pendingAttempt
	var status string
	err := queryer.QueryRow(ctx, lockAttemptQuery, attemptID).
		Scan(&attempt.caseID, &attempt.revision, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return pendingAttempt{}, ErrAttemptNotFound
	}
	if err != nil {
		return pendingAttempt{}, fmt.Errorf("lock scoring attempt: %w", err)
	}
	if status != "PENDING" {
		return pendingAttempt{}, ErrAttemptCompleted
	}
	return attempt, nil
}
