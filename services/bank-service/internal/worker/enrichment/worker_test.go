package enrichmentworker

import (
	"context"
	"errors"
	"testing"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	enrichmentrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/enrichment"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type queueStub struct {
	claim     enrichmentrepository.Claim
	outcome   enrichmentrepository.Outcome
	completed bool
	released  bool
	err       error
}

func (q *queueStub) ClaimNext(context.Context) (enrichmentrepository.Claim, bool, error) {
	return q.claim, true, nil
}

func (q *queueStub) Complete(_ context.Context, _ enrichmentrepository.Claim, outcome enrichmentrepository.Outcome) (bool, error) {
	q.completed, q.outcome = true, outcome
	return q.err == nil, q.err
}

func (q *queueStub) Release(context.Context, enrichmentrepository.Claim) error {
	q.released = true
	return nil
}

type bureauStub struct {
	profileErr error
	creditErr  error
	inns       []string
}

func (b *bureauStub) GetCompanyProfile(_ context.Context, inn string) (*bureaupb.CompanyProfileResponse, error) {
	b.inns = append(b.inns, inn)
	if b.profileErr != nil {
		return nil, b.profileErr
	}
	return &bureaupb.CompanyProfileResponse{Status: bureaupb.CompanyStatus_COMPANY_STATUS_ACTIVE, RegisteredAt: "2019-04-15"}, nil
}

func (b *bureauStub) GetCreditHistory(_ context.Context, inn string) (*bureaupb.CreditHistoryResponse, error) {
	b.inns = append(b.inns, inn)
	if b.creditErr != nil {
		return nil, b.creditErr
	}
	return &bureaupb.CreditHistoryResponse{}, nil
}

func TestWorkerRecordsBothChecksAndRoutesError(t *testing.T) {
	for _, tc := range []struct {
		name       string
		creditErr  error
		wantCredit string
	}{
		{name: "success"},
		{name: "timeout", creditErr: status.Error(codes.DeadlineExceeded, "timeout"), wantCredit: "BUREAU_TIMEOUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue := &queueStub{claim: enrichmentrepository.Claim{CaseID: uuid.New(), INN: "7707083893"}}
			bureau := &bureauStub{creditErr: tc.creditErr}
			worked, err := New(queue, bureau).RunOnce(t.Context())
			if err != nil || !worked || !queue.completed || queue.released ||
				queue.outcome.Profile == nil || queue.outcome.ProfileError != "" ||
				queue.outcome.CreditError != tc.wantCredit || len(bureau.inns) != 2 ||
				bureau.inns[0] != queue.claim.INN || bureau.inns[1] != queue.claim.INN {
				t.Fatalf("worked=%t err=%v outcome=%+v released=%t inns=%v",
					worked, err, queue.outcome, queue.released, bureau.inns)
			}
			if tc.creditErr == nil && queue.outcome.Credit == nil || tc.creditErr != nil && queue.outcome.Credit != nil {
				t.Fatalf("credit response mismatch: %+v", queue.outcome)
			}
		})
	}
}

func TestWorkerReleasesAfterPersistenceFailure(t *testing.T) {
	queue := &queueStub{claim: enrichmentrepository.Claim{CaseID: uuid.New(), INN: "7707083893"}, err: errors.New("database unavailable")}
	worked, err := New(queue, &bureauStub{}).RunOnce(t.Context())
	if err == nil || !worked || !queue.released {
		t.Fatalf("worked=%t err=%v released=%t", worked, err, queue.released)
	}
}
