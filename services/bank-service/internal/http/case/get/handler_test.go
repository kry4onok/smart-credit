package gethandler

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
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	"github.com/google/uuid"
)

func TestHandlerHandle(t *testing.T) {
	caseID := uuid.New()
	productID := uuid.New()
	currentUserID := uuid.New()
	createdAt := time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC)

	handler := New(useCaseMock{execute: func(
		_ context.Context,
		input getcase.Input,
	) (getcase.Output, error) {
		if input.CaseID != caseID || input.CurrentUserID != currentUserID {
			t.Fatalf("input = %+v", input)
		}

		return getcase.Output{
			Case: model.Case{
				ID:             caseID,
				Status:         model.CaseStatusManualReview,
				ProductID:      productID,
				ProductName:    "Оборотный кредит",
				ProductVersion: 2,
				Company: model.Company{
					INN:          "7707083893",
					Name:         "ООО Тест",
					RegisteredAt: createdAt,
				},
				Application: model.SubmittedApplication{
					AmountKopecks: 5_000_000,
					TermMonths:    12,
				},
				CreatedAt:    createdAt,
				UpdatedAt:    createdAt,
				AuditVersion: 4,
				Version:      7,
			},
			AllowedActions: []model.CaseAction{model.CaseActionView},
		}, nil
	}})

	ctx := authmiddleware.WithUserID(context.Background(), currentUserID)
	response, err := handler.Handle(ctx, generated.GetCaseRequestObject{ApplicationId: caseID})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	result, ok := response.(generated.GetCase200JSONResponse)
	if !ok {
		t.Fatalf("response type = %T", response)
	}
	if result.Body.Id != caseID || result.Body.Application.Amount.AmountKopecks != 5_000_000 {
		t.Fatalf("response body = %+v", result.Body)
	}
	if result.Body.Application.Amount.Currency != common.MoneyCurrencyRUB {
		t.Fatalf("currency = %q, want RUB", result.Body.Application.Amount.Currency)
	}
	if result.Headers.ETag == nil || *result.Headers.ETag != "\"7\"" {
		t.Fatalf("ETag = %v, want quoted version", result.Headers.ETag)
	}
}

func TestHandlerHandleMapsNotFound(t *testing.T) {
	handler := New(useCaseMock{execute: func(
		context.Context,
		getcase.Input,
	) (getcase.Output, error) {
		return getcase.Output{}, getcase.ErrNotFound
	}})

	_, err := handler.Handle(context.Background(), generated.GetCaseRequestObject{
		ApplicationId: uuid.New(),
	})
	var httpErr *domainerror.Error
	if !errors.As(err, &httpErr) {
		t.Fatalf("Handle() error = %v, want domain error", err)
	}
	if httpErr.Status != http.StatusNotFound || httpErr.Code != "CASE_NOT_FOUND" {
		t.Fatalf("domain error = %+v", httpErr)
	}
}
