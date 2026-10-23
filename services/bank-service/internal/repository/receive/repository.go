package receiverepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrEventConflict = errors.New("client event ID or aggregate version conflicts with a different event")
var ErrInfoProvidedConflict = errors.New("InfoProvided does not match an open request on a NEEDS_INFO case")

type Repository struct {
	db transaction.Queryer
}

func New(db transaction.Queryer) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Apply(ctx context.Context, input receive.Input) (bool, error) {
	q := transaction.QueryerFromContext(ctx, r.db)
	var reserved int
	err := q.QueryRow(ctx, reserveEventQuery,
		input.EventID, input.ApplicationID, input.AggregateVersion).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		var aggregateID uuid.UUID
		var version int64
		err = q.QueryRow(ctx, selectDuplicateEventQuery, input.EventID).
			Scan(&aggregateID, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrEventConflict
		}
		if err != nil {
			return false, fmt.Errorf("read duplicate event: %w", err)
		}
		if aggregateID != input.ApplicationID || version != input.AggregateVersion {
			return false, ErrEventConflict
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reserve event: %w", err)
	}
	if input.InfoProvided {
		return r.applyInfoProvided(ctx, q, input)
	}

	status := "VALIDATION"
	var recommendation *string
	var scheduleValidation bool
	var triggeredRules []string
	var productName string
	err = q.QueryRow(ctx, selectProductQuery,
		input.ProductID, input.ProductVersion).Scan(&productName)
	if errors.Is(err, pgx.ErrNoRows) {
		productName = "Неизвестный продукт"
		status = "MANUAL_REVIEW"
		manualReview := "MANUAL_REVIEW"
		recommendation = &manualReview
		triggeredRules = []string{"UNKNOWN_PRODUCT"}
	} else if err != nil {
		return false, fmt.Errorf("load product: %w", err)
	} else {
		scheduleValidation = true
		triggeredRules = []string{}
	}

	_, err = q.Exec(ctx, insertCaseQuery,
		input.ApplicationID, input.OrgID, input.ClientUserID,
		input.ProductID, input.ProductVersion, productName,
		input.CompanyINN, input.CompanyName, string(input.CompanyJSON), string(input.FormJSON),
		string(input.DocumentsJSON), input.AmountKopecks, input.TermMonths,
		input.PurposeTag, status, recommendation, triggeredRules, scheduleValidation,
		input.AggregateVersion)
	if err != nil {
		return false, fmt.Errorf("create case: %w", err)
	}

	_, err = q.Exec(ctx, insertAuditQuery,
		uuid.New(), input.ApplicationID, input.ClientUserID,
		reason(status), input.EventID.String())
	if err != nil {
		return false, fmt.Errorf("audit submitted application: %w", err)
	}
	return true, nil
}

func (r *Repository) applyInfoProvided(ctx context.Context, q transaction.Queryer, input receive.Input) (bool, error) {
	var status string
	var clientEventVersion int64
	err := q.QueryRow(ctx, selectCaseForInfoQuery, input.ApplicationID).Scan(&status, &clientEventVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrInfoProvidedConflict
	}
	if err != nil {
		return false, fmt.Errorf("lock case for InfoProvided: %w", err)
	}
	if status != "NEEDS_INFO" || input.AggregateVersion <= clientEventVersion {
		return false, ErrInfoProvidedConflict
	}
	var requestID uuid.UUID
	err = q.QueryRow(ctx, selectOpenInfoRequestQuery, input.ApplicationID).Scan(&requestID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && requestID != input.RequestID) {
		return false, ErrInfoProvidedConflict
	}
	if err != nil {
		return false, fmt.Errorf("lock open info request: %w", err)
	}
	result, err := q.Exec(ctx, fulfillInfoRequestQuery, input.RequestID, input.ApplicationID)
	if err != nil {
		return false, fmt.Errorf("fulfill info request: %w", err)
	}
	if result.RowsAffected() != 1 {
		return false, ErrInfoProvidedConflict
	}
	var version int64
	err = q.QueryRow(ctx, updateCaseFromInfoQuery,
		input.ApplicationID, input.CompanyINN, input.CompanyName,
		string(input.CompanyJSON), string(input.FormJSON), string(input.DocumentsJSON),
		input.AmountKopecks, input.TermMonths, input.PurposeTag, input.AggregateVersion).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrInfoProvidedConflict
	}
	if err != nil {
		return false, fmt.Errorf("update case from InfoProvided: %w", err)
	}
	_, err = q.Exec(ctx, insertInfoProvidedAuditQuery, uuid.New(), input.ApplicationID, version,
		input.EventID.String(), input.RequestID.String())
	if err != nil {
		return false, fmt.Errorf("audit InfoProvided: %w", err)
	}
	return true, nil
}

func reason(status string) string {
	if status == "MANUAL_REVIEW" {
		return "UNKNOWN_PRODUCT"
	}
	return ""
}
