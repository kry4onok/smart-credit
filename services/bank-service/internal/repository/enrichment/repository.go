package enrichmentrepository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Claim struct {
	CaseID        uuid.UUID
	INN           string
	Version       int64
	InputRevision int64
	LeaseUntil    time.Time
}

type Outcome struct {
	Profile      *bureaupb.CompanyProfileResponse
	ProfileError string
	Credit       *bureaupb.CreditHistoryResponse
	CreditError  string
}

type Repository struct {
	db           transaction.Queryer
	transactions *transaction.Manager
}

func New(db transaction.Queryer, transactions *transaction.Manager) *Repository {
	return &Repository{db: db, transactions: transactions}
}

func (r *Repository) ClaimNext(ctx context.Context) (Claim, bool, error) {
	var claim Claim
	err := r.db.QueryRow(ctx, claimNextQuery).Scan(
		&claim.CaseID, &claim.INN, &claim.Version, &claim.InputRevision, &claim.LeaseUntil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Claim{}, false, nil
	}
	if err != nil {
		return Claim{}, false, fmt.Errorf("claim enrichment case: %w", err)
	}
	return claim, true, nil
}

func (r *Repository) Complete(ctx context.Context, claim Claim, outcome Outcome) (bool, error) {
	var profileMessage, creditMessage proto.Message
	if outcome.Profile != nil {
		profileMessage = outcome.Profile
	}
	if outcome.Credit != nil {
		creditMessage = outcome.Credit
	}
	company, err := prepareSource(profileMessage, outcome.ProfileError)
	if err != nil {
		return false, fmt.Errorf("prepare company result: %w", err)
	}
	credit, err := prepareSource(creditMessage, outcome.CreditError)
	if err != nil {
		return false, fmt.Errorf("prepare credit result: %w", err)
	}
	status, reason := "SCORING", ""
	var recommendation any
	if company.errorCode != "" || credit.errorCode != "" {
		status, reason, recommendation = "MANUAL_REVIEW", "BUREAU_UNAVAILABLE", "MANUAL_REVIEW"
	}
	details, err := json.Marshal(map[string]string{
		"to_status": status, "company_error_code": company.errorCode,
		"credit_error_code": credit.errorCode,
	})
	if err != nil {
		return false, fmt.Errorf("encode enrichment audit: %w", err)
	}
	var applied bool
	err = r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		queryer := transaction.QueryerFromContext(ctx, r.db)
		var version int64
		err := queryer.QueryRow(ctx, finishCaseQuery,
			claim.CaseID, claim.LeaseUntil, claim.Version, claim.InputRevision,
			status, recommendation,
		).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("finish enrichment case: %w", err)
		}
		for _, item := range []struct {
			source string
			value  sourceResult
		}{
			{source: "COMPANY", value: company},
			{source: "CREDIT_HISTORY", value: credit},
		} {
			_, err := queryer.Exec(ctx, insertResultQuery, claim.CaseID, item.source,
				item.value.outcome, item.value.resultJSON, nullable(item.value.errorCode))
			if err != nil {
				return fmt.Errorf("save %s enrichment result: %w", item.source, err)
			}
		}
		_, err = queryer.Exec(ctx, insertAuditQuery, uuid.New(), claim.CaseID, version, reason, string(details))
		if err != nil {
			return fmt.Errorf("audit enrichment transition: %w", err)
		}
		applied = true
		return nil
	})
	return applied, err
}

type sourceResult struct {
	outcome    string
	resultJSON any
	errorCode  string
}

func prepareSource(value proto.Message, errorCode string) (sourceResult, error) {
	if (value == nil) == (errorCode == "") {
		return sourceResult{}, errors.New("exactly one of response or error code is required")
	}
	if errorCode != "" {
		return sourceResult{outcome: "ERROR", errorCode: errorCode}, nil
	}
	data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(value)
	if err != nil {
		return sourceResult{}, fmt.Errorf("encode protobuf response: %w", err)
	}
	return sourceResult{outcome: "SUCCESS", resultJSON: string(data)}, nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *Repository) Release(ctx context.Context, claim Claim) error {
	_, err := r.db.Exec(ctx, releaseQuery, claim.CaseID, claim.LeaseUntil)
	if err != nil {
		return fmt.Errorf("release enrichment case: %w", err)
	}
	return nil
}
