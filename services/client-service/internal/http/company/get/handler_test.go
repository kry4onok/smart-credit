package get

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	profileconv "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func TestCompanyUsesAuthenticatedOrganization(t *testing.T) {
	orgID, otherID := uuid.New(), uuid.New()
	company := model.Company{INN: "7707083893", Name: "Демо", OKVED: "62.01",
		RegisteredAt:      time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
		Revenue12mKopecks: profileconv.Ptr(int64(9007199254740991)), Profit12mKopecks: profileconv.Ptr(int64(-12345)), ObligationsKopecks: profileconv.Ptr(int64(500)), Version: 7}
	calls := 0
	h := New(usecaseMock{execute: func(_ context.Context, id uuid.UUID) (model.Company, error) {
		calls++
		if id != orgID {
			t.Fatalf("tenant = %s, want %s", id, orgID)
		}
		return company, nil
	}})
	endpoint := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, err := h.Handle(r.Context(), generated.GetCompanyRequestObject{})
		if err != nil {
			t.Fatal(err)
		}
		if err := response.VisitGetCompanyResponse(w); err != nil {
			t.Fatal(err)
		}
	})
	handler := domainerror.Trace(middleware.RequireClient(verifierMock{model.Principal{
		UserID: uuid.New(), OrgID: orgID, Role: "CLIENT", CanSubmit: false,
	}}, endpoint))
	request := httptest.NewRequest(http.MethodGet, "/client/company?org_id="+otherID.String(), nil)
	request.Header.Set("Authorization", "Bearer test")
	request.Header.Set("X-Org-ID", otherID.String())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || calls != 1 {
		t.Fatalf("status = %d, calls = %d: %s", response.Code, calls, response.Body.String())
	}
	if response.Header().Get("ETag") != `"7"` || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("headers = %v", response.Header())
	}
	var body generated.VersionedCompany
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Version != 7 || body.Profile.Inn != company.INN || body.Profile.RegisteredAt.String() != "2020-01-02" ||
		body.Profile.Revenue12m.AmountKopecks != *company.Revenue12mKopecks || body.Profile.Profit12m.AmountKopecks != -12345 ||
		body.Profile.Obligations.Currency != common.MoneyCurrencyRUB {
		t.Fatalf("unexpected profile: %+v", body)
	}
}

func TestCompanyErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"missing", domain.ErrCompanyNotFound, http.StatusNotFound, "COMPANY_NOT_FOUND"},
		{"database failure", errors.New("secret database details"), http.StatusInternalServerError, "INTERNAL_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := New(usecaseMock{execute: func(context.Context, uuid.UUID) (model.Company, error) {
				return model.Company{}, tc.err
			}})
			handler := domainerror.Trace(middleware.RequireClient(verifierMock{model.Principal{
				UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT",
			}}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response, err := h.Handle(r.Context(), generated.GetCompanyRequestObject{})
				if err != nil {
					t.Fatal(err)
				}
				if err := response.VisitGetCompanyResponse(w); err != nil {
					t.Fatal(err)
				}
			})))
			request := httptest.NewRequest(http.MethodGet, "/client/company", nil)
			request.Header.Set("Authorization", "Bearer test")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var problem common.Problem
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if response.Code != tc.status || problem.Code != tc.code || problem.Status != tc.status ||
				problem.TraceId != response.Header().Get("X-Trace-ID") || problem.TraceId == "" ||
				problem.Detail == nil || *problem.Detail == "secret database details" ||
				response.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("status = %d, problem = %+v", response.Code, problem)
			}
		})
	}
}

func TestCompanyWithoutPrincipalDoesNotCallUsecase(t *testing.T) {
	h := New(usecaseMock{execute: func(context.Context, uuid.UUID) (model.Company, error) {
		t.Fatal("unauthenticated request reached usecase")
		return model.Company{}, nil
	}})
	response, err := h.Handle(context.Background(), generated.GetCompanyRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetCompanyResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}
