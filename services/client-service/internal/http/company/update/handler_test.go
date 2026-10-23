package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type usecaseMock struct {
	company model.Company
	version *int64
}

func (m *usecaseMock) Execute(_ context.Context, _ uuid.UUID, _ model.Company, version *int64) (model.Company, error) {
	m.version = version
	return m.company, nil
}

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}

func TestHandleCreatesCompanyAndReturnsETag(t *testing.T) {
	usecase := &usecaseMock{company: model.Company{INN: "7707083893", Name: "Демо", OKVED: "62.01", RegisteredAt: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), Version: 1}}
	handler := New(usecase)
	body := common.CompanyProfile{Inn: "7707083893", Name: "Демо", Okved: "62.01", RegisteredAt: openapi_types.Date{Time: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)}, Revenue12m: common.Money{Currency: common.MoneyCurrencyRUB}, Profit12m: common.SignedMoney{Currency: common.SignedMoneyCurrencyRUB}, Obligations: common.Money{Currency: common.MoneyCurrencyRUB}}
	request := generated.UpdateCompanyRequestObject{Body: &body}
	recorder := serve(t, handler, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("ETag") != `"1"` {
		t.Fatalf("status = %d, headers = %v", recorder.Code, recorder.Header())
	}
	if usecase.version != nil {
		t.Fatalf("unexpected version = %v", *usecase.version)
	}
}

func TestHandleParsesIfMatchAndRejectsMalformedValue(t *testing.T) {
	usecase := &usecaseMock{company: model.Company{Version: 2}}
	handler := New(usecase)
	body := common.CompanyProfile{Inn: "7707083893", Name: "Демо", Okved: "62.01", RegisteredAt: openapi_types.Date{Time: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)}, Revenue12m: common.Money{Currency: common.MoneyCurrencyRUB}, Profit12m: common.SignedMoney{Currency: common.SignedMoneyCurrencyRUB}, Obligations: common.Money{Currency: common.MoneyCurrencyRUB}}
	value := `"7"`
	serve(t, handler, generated.UpdateCompanyRequestObject{Params: generated.UpdateCompanyParams{IfMatch: &value}, Body: &body})
	if usecase.version == nil || *usecase.version != 7 {
		t.Fatalf("version = %v", usecase.version)
	}
	for _, bad := range []string{"7", `"+7"`} {
		recorder := serve(t, handler, generated.UpdateCompanyRequestObject{Params: generated.UpdateCompanyParams{IfMatch: &bad}, Body: &body})
		var problem common.Problem
		if json.Unmarshal(recorder.Body.Bytes(), &problem) != nil || recorder.Code != http.StatusBadRequest || problem.Code != "INVALID_IF_MATCH" {
			t.Fatalf("if-match = %q, status = %d, body = %s", bad, recorder.Code, recorder.Body.String())
		}
	}
}

func serve(t *testing.T, handler *Handler, request generated.UpdateCompanyRequestObject) *httptest.ResponseRecorder {
	t.Helper()
	endpoint := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.Handle(r.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		if err := response.VisitUpdateCompanyResponse(w); err != nil {
			t.Fatal(err)
		}
	})
	protected := domainerror.Trace(middleware.RequireClient(verifierMock{model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}}, endpoint))
	recorder := httptest.NewRecorder()
	requestHTTP := httptest.NewRequest(http.MethodPut, "/client/company", nil)
	requestHTTP.Header.Set("Authorization", "Bearer test")
	protected.ServeHTTP(recorder, requestHTTP)
	return recorder
}
