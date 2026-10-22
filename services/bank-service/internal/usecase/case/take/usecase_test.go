package take

import (
	"context"
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

type transactionStub struct {
	calls     int
	committed bool
}

func (s *transactionStub) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	s.calls++
	err := fn(ctx)
	s.committed = err == nil
	return err
}

type repositoryStub struct {
	result model.Case
	err    error
	calls  int
}

func (s *repositoryStub) Take(context.Context, Input) (model.Case, error) {
	s.calls++
	return s.result, s.err
}

func TestTakeTransaction(t *testing.T) {
	result := model.Case{ID: uuid.New(), Version: 2}
	repository := &repositoryStub{result: result}
	transactions := &transactionStub{}
	got, err := New(repository, transactions).Execute(t.Context(), Input{})
	if err != nil || got.ID != result.ID || transactions.calls != 1 || !transactions.committed {
		t.Fatalf("result=%#v err=%v transaction=%#v", got, err, transactions)
	}

	repository.err = ErrConflict
	_, err = New(repository, transactions).Execute(t.Context(), Input{})
	if !errors.Is(err, ErrConflict) || transactions.committed {
		t.Fatalf("err=%v transaction=%#v; want rollback", err, transactions)
	}
}
