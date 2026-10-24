package decline

import (
	"context"
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func TestExecuteRejectsInvalidAction(t *testing.T) {
	params := model.DeclineOfferParams{
		OrgID: uuid.New(), UserID: uuid.New(), ApplicationID: uuid.New(), OfferID: uuid.New(),
		IdempotencyID: uuid.New(), RequestHash: make([]byte, 32),
	}
	params.OfferID = uuid.Nil

	_, err := New(repositoryMock{}).Execute(context.Background(), params)
	if !errors.Is(err, domain.ErrInvalidOfferAction) {
		t.Fatalf("err = %v", err)
	}
}

type repositoryMock struct{}

func (repositoryMock) DeclineOffer(context.Context, model.DeclineOfferParams) (model.ActionAccepted, error) {
	return model.ActionAccepted{}, nil
}
