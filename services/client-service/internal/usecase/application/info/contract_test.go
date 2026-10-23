package info

import (
	"context"
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func TestExecuteRejectsEmptyResponse(t *testing.T) {
	params := validParams()
	params.Company = nil
	params.Fields = nil
	params.DocumentsSpecified = false
	_, err := New(repositoryMock{}).Execute(context.Background(), params)
	if !errors.Is(err, domain.ErrInvalidInfoResponse) {
		t.Fatalf("err = %v", err)
	}
}

func validParams() model.ProvideInfoParams {
	hash := make([]byte, 32)
	return model.ProvideInfoParams{
		OrgID: uuid.New(), UserID: uuid.New(), ApplicationID: uuid.New(),
		RequestID: uuid.New(), IdempotencyID: uuid.New(), RequestHash: hash,
		DocumentsSpecified: true,
	}
}

type repositoryMock struct{}

func (repositoryMock) ProvideInfo(context.Context, model.ProvideInfoParams) (model.ActionAccepted, error) {
	return model.ActionAccepted{}, nil
}
