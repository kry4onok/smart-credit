package cancel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type repositoryMock struct{ called bool }

func (m *repositoryMock) CancelApplication(_ context.Context, params model.CancelApplicationParams) (model.ActionAccepted, error) {
	m.called = true
	return model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "CANCEL_PENDING"}, nil
}

func TestExecuteValidatesCancellation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*model.CancelApplicationParams)
		valid  bool
	}{
		{"without reason", func(*model.CancelApplicationParams) {}, true},
		{"500 unicode characters", func(p *model.CancelApplicationParams) { p.Reason = strings.Repeat("я", 500) }, true},
		{"too long reason", func(p *model.CancelApplicationParams) { p.Reason = strings.Repeat("я", 501) }, false},
		{"invalid utf8", func(p *model.CancelApplicationParams) { p.Reason = string([]byte{0xff}) }, false},
		{"missing org", func(p *model.CancelApplicationParams) { p.OrgID = uuid.Nil }, false},
		{"missing user", func(p *model.CancelApplicationParams) { p.UserID = uuid.Nil }, false},
		{"missing application", func(p *model.CancelApplicationParams) { p.ApplicationID = uuid.Nil }, false},
		{"missing key", func(p *model.CancelApplicationParams) { p.IdempotencyID = uuid.Nil }, false},
		{"missing hash", func(p *model.CancelApplicationParams) { p.RequestHash = nil }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := model.CancelApplicationParams{OrgID: uuid.New(), UserID: uuid.New(), ApplicationID: uuid.New(), IdempotencyID: uuid.New(), RequestHash: make([]byte, 32)}
			tc.mutate(&params)
			repo := &repositoryMock{}
			_, err := New(repo).Execute(t.Context(), params)
			if tc.valid && (err != nil || !repo.called) {
				t.Fatalf("err=%v, repository called=%v", err, repo.called)
			}
			if !tc.valid && (!errors.Is(err, domain.ErrInvalidCancelAction) || repo.called) {
				t.Fatalf("err=%v, repository called=%v", err, repo.called)
			}
		})
	}
}
