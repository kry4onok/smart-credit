package listhandler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
	"github.com/google/uuid"
)

func TestHandlerHandle(t *testing.T) {
	currentUserID := uuid.New()
	caseID := uuid.New()
	productID := uuid.New()
	limit := 10
	assignee := "me"
	status := common.BankStatus(model.CaseStatusManualReview)
	createdAt := time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC)

	handler := New(useCaseMock{execute: func(
		_ context.Context,
		input listcase.Input,
	) (listcase.Output, error) {
		if input.CurrentUserID != currentUserID || input.Status == nil ||
			*input.Status != model.CaseStatusManualReview {
			t.Fatalf("input = %+v", input)
		}

		return listcase.Output{Items: []model.CaseListItem{{
			ID:                     caseID,
			Status:                 model.CaseStatusManualReview,
			ProductID:              productID,
			RequestedAmountKopecks: 5_000_000,
			CompanyName:            "ООО Тлижу",
			CreatedAt:              createdAt,
			Version:                3,
		}}}, nil
	}})

	ctx := authmiddleware.WithUserID(context.Background(), currentUserID)
	response, err := handler.Handle(ctx, generated.ListCasesRequestObject{
		Params: generated.ListCasesParams{
			Limit:    &limit,
			Status:   &status,
			Assignee: &assignee,
		},
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	result, ok := response.(generated.ListCases200JSONResponse)
	if !ok {
		t.Fatalf("response type = %T", response)
	}
	if len(result.Items) != 1 || result.Items[0].Id != caseID {
		t.Fatalf("response = %+v", result)
	}
	if result.Items[0].RequestedAmount.Currency != common.MoneyCurrencyRUB {
		t.Fatalf("currency = %q, want RUB", result.Items[0].RequestedAmount.Currency)
	}
}

func TestHandlerHandleMapsInvalidFilter(t *testing.T) {
	handler := New(useCaseMock{execute: func(
		context.Context,
		listcase.Input,
	) (listcase.Output, error) {
		return listcase.Output{}, listcase.ErrInvalidCursor
	}})

	_, err := handler.Handle(context.Background(), generated.ListCasesRequestObject{})
	var httpErr *domainerror.Error
	if !errors.As(err, &httpErr) {
		t.Fatalf("Handle() error = %v, want domain error", err)
	}
	if httpErr.Status != http.StatusBadRequest || httpErr.Code != "INVALID_LIST_FILTER" {
		t.Fatalf("domain error = %+v", httpErr)
	}
}
