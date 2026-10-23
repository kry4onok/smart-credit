package update

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type repositoryMock struct {
	created, updated bool
}

func (r *repositoryMock) Create(context.Context, uuid.UUID, model.Company) (model.Company, error) {
	r.created = true
	return model.Company{Version: 1}, nil
}

func (r *repositoryMock) Update(context.Context, uuid.UUID, model.Company, int64) (model.Company, error) {
	r.updated = true
	return model.Company{Version: 2}, nil
}

func validCompany() model.Company {
	return model.Company{INN: "7707083893", Name: "Демо", OKVED: "62.01", RegisteredAt: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)}
}

func TestExecuteChoosesCreateOrUpdate(t *testing.T) {
	repository := &repositoryMock{}
	service := New(repository)
	if _, err := service.Execute(context.Background(), uuid.New(), validCompany(), nil); err != nil || !repository.created {
		t.Fatalf("create: err = %v, created = %v", err, repository.created)
	}
	if _, err := service.Execute(context.Background(), uuid.New(), validCompany(), ptr(int64(1))); err != nil || !repository.updated {
		t.Fatalf("update: err = %v, updated = %v", err, repository.updated)
	}
}

func TestExecuteRejectsInvalidProfileBeforeRepository(t *testing.T) {
	repository := &repositoryMock{}
	invalid := validCompany()
	invalid.INN = "7707083894"
	_, err := New(repository).Execute(context.Background(), uuid.New(), invalid, nil)
	if !errors.Is(err, domain.ErrInvalidCompanyProfile) || repository.created || repository.updated {
		t.Fatalf("err = %v, repository called: created=%v updated=%v", err, repository.created, repository.updated)
	}
}

func ptr(value int64) *int64 { return &value }
