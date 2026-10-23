package scoringrepository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	scoringclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/scoring"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
)

var ErrInputNotReady = errors.New("latest company and credit checks must both be successful")

type SavedInput struct {
	Input    scoringclient.Input
	Revision int64
}


func (r *Repository) LoadInput(ctx context.Context, caseID uuid.UUID) (SavedInput, error) {
	if caseID == uuid.Nil {
		return SavedInput{}, errors.New("invalid scoring case ID")
	}
	var productID uuid.UUID
	var revision int64
	var companyJSON, profileJSON, creditJSON []byte
	var profileOutcome, creditOutcome *string
	err := r.db.QueryRow(ctx, loadInputQuery, caseID).Scan(
		&productID, &revision, &companyJSON,
		&profileOutcome, &profileJSON, &creditOutcome, &creditJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return SavedInput{}, ErrCaseNotFound
	}
	if err != nil {
		return SavedInput{}, fmt.Errorf("load scoring sources: %w", err)
	}
	if profileOutcome == nil || *profileOutcome != "SUCCESS" ||
		creditOutcome == nil || *creditOutcome != "SUCCESS" {
		return SavedInput{}, ErrInputNotReady
	}
	input, err := decodeSavedInput(caseID, productID, companyJSON, profileJSON, creditJSON)
	if err != nil {
		return SavedInput{}, err
	}
	return SavedInput{Input: input, Revision: revision}, nil
}

func decodeSavedInput(caseID, productID uuid.UUID, companyJSON, profileJSON, creditJSON []byte) (scoringclient.Input, error) {
	company := &commonpb.CompanySnapshot{}
	if err := protojson.Unmarshal(companyJSON, company); err != nil {
		return scoringclient.Input{}, fmt.Errorf("decode company snapshot: %w", err)
	}

	var profileSource struct {
		RegisteredAt string `json:"registered_at"`
	}
	if err := json.Unmarshal(profileJSON, &profileSource); err != nil {
		return scoringclient.Input{}, fmt.Errorf("decode company check: %w", err)
	}
	credit := &bureaupb.CreditHistoryResponse{}
	if err := protojson.Unmarshal(creditJSON, credit); err != nil {
		return scoringclient.Input{}, fmt.Errorf("decode credit history: %w", err)
	}
	return scoringclient.Input{
		ApplicationID: caseID, ProductID: productID, Company: company,
		Profile:       &bureaupb.CompanyProfileResponse{RegisteredAt: profileSource.RegisteredAt},
		CreditHistory: credit,
	}, nil
}
