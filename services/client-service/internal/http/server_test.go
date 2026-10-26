package httpapi

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	applicationcancel "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/cancel"
	applicationcreate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/create"
	applicationdocument "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/document"
	applicationget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/get"
	applicationinfo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/info"
	applicationlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/list"
	applicationofferaccept "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/offer/accept"
	applicationofferdecline "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/offer/decline"
	applicationsign "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/sign"
	applicationsubmit "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/submit"
	applicationtimeline "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/timeline"
	applicationupdate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/update"
	companyget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/get"
	productget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/get"
	productlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/list"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	listapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
	"github.com/google/uuid"
)

type pingerMock struct{}

func (pingerMock) Ping(context.Context) error { return nil }

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}

type companyMock struct{}

func (companyMock) Execute(context.Context, uuid.UUID) (model.Company, error) {
	return model.Company{INN: "7707083893", Name: "Demo", OKVED: "62.01", Version: 1, RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
}

type productsMock struct{}

func (productsMock) Execute(context.Context, *string) (model.ProductCatalog, error) {
	return model.ProductCatalog{Version: 1, Items: []model.Product{}}, nil
}

type productMock struct{}

func (productMock) Execute(_ context.Context, id uuid.UUID) (model.Product, error) {
	return model.Product{ID: id, Version: 1, Code: "EXPRESS", Name: "Экспресс"}, nil
}

type applicationMock struct{}

func (applicationMock) Execute(_ context.Context, params model.CreateApplicationParams) (model.Application, error) {
	return model.Application{ID: uuid.New(), ProductID: params.ProductID, ProductVersion: 1, Company: model.Company{
		INN: "7707083893", Name: "Demo", OKVED: "62.01", Version: 1,
		RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}, Status: "DRAFT", StageLabel: "Черновик", NextAction: "Заполните заявку", Version: 1}, nil
}

type applicationGetMock struct{}

func (applicationGetMock) Execute(_ context.Context, _ uuid.UUID, applicationID uuid.UUID) (model.Application, error) {
	return model.Application{ID: applicationID, ProductID: uuid.New(), ProductVersion: 1, Company: model.Company{
		INN: "7707083893", Name: "Demo", OKVED: "62.01",
		RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}, Status: "DRAFT", StageLabel: "Черновик", NextAction: "Заполните заявку", Version: 1}, nil
}

type applicationUpdateMock struct{}

func (applicationUpdateMock) Execute(_ context.Context, params model.UpdateApplicationParams) (model.Application, error) {
	return model.Application{ID: params.ApplicationID, ProductID: uuid.New(), ProductVersion: 1, Company: model.Company{
		INN: "7707083893", Name: "Demo", OKVED: "62.01",
		RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}, Status: "DRAFT", StageLabel: "Черновик", NextAction: "Заполните заявку", Version: 2}, nil
}

type applicationListMock struct{}

func (applicationListMock) Execute(_ context.Context, _ listapplication.Input) (listapplication.Output, error) {
	return listapplication.Output{}, nil
}

type applicationSubmitMock struct{}

func (applicationSubmitMock) Execute(_ context.Context, params model.SubmitApplicationParams) (model.Application, error) {
	return model.Application{ID: params.ApplicationID, ProductID: uuid.New(), ProductVersion: 1, Company: model.Company{
		INN: "7707083893", Name: "Demo", OKVED: "62.01",
		RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}, Status: "SUBMITTED", StageLabel: "Отправлена", NextAction: "Ожидайте решения банка", Version: 2}, nil
}

type applicationDocumentMock struct{}

func (applicationDocumentMock) Execute(_ context.Context, _ model.AddApplicationDocumentParams) (model.ApplicationDocument, error) {
	return model.ApplicationDocument{ID: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "statement.pdf", Sha256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", IsMock: true, UploadedAt: time.Now().UTC()}, nil
}

type applicationInfoMock struct{}

func (applicationInfoMock) Execute(_ context.Context, params model.ProvideInfoParams) (model.ActionAccepted, error) {
	return model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "INFO_SENT"}, nil
}

type applicationOfferAcceptMock struct{}

func (applicationOfferAcceptMock) Execute(_ context.Context, params model.AcceptOfferParams) (model.ActionAccepted, error) {
	return model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "ACCEPT_PENDING"}, nil
}

type applicationOfferDeclineMock struct{}

func (applicationOfferDeclineMock) Execute(_ context.Context, params model.DeclineOfferParams) (model.ActionAccepted, error) {
	return model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "DECLINE_PENDING"}, nil
}

type applicationSignMock struct{}

func (applicationSignMock) Execute(_ context.Context, params model.SignContractParams) (model.ActionAccepted, error) {
	return model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "SIGN_PENDING"}, nil
}

type applicationTimelineMock struct{}

func (applicationTimelineMock) Execute(context.Context, uuid.UUID, uuid.UUID) ([]model.TimelineEntry, error) {
	return []model.TimelineEntry{}, nil
}

type applicationCancelMock struct{}

func (applicationCancelMock) Execute(_ context.Context, params model.CancelApplicationParams) (model.ActionAccepted, error) {
	return model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "CANCEL_PENDING"}, nil
}

func TestRoutes(t *testing.T) {
	handler := New(Params{
		DB:                      pingerMock{},
		ReadinessTimeout:        time.Second,
		Verifier:                verifierMock{model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT", CanSubmit: true}},
		CompanyGet:              companyget.New(companyMock{}),
		ApplicationCreate:       applicationcreate.New(applicationMock{}),
		ApplicationCancel:       applicationcancel.New(applicationCancelMock{}),
		ApplicationDocument:     applicationdocument.New(applicationDocumentMock{}),
		ApplicationGet:          applicationget.New(applicationGetMock{}),
		ApplicationList:         applicationlist.New(applicationListMock{}),
		ApplicationTimeline:     applicationtimeline.New(applicationTimelineMock{}),
		ApplicationOfferAccept:  applicationofferaccept.New(applicationOfferAcceptMock{}),
		ApplicationOfferDecline: applicationofferdecline.New(applicationOfferDeclineMock{}),
		ApplicationSign:         applicationsign.New(applicationSignMock{}),
		ApplicationSubmit:       applicationsubmit.New(applicationSubmitMock{}),
		ApplicationInfo:         applicationinfo.New(applicationInfoMock{}),
		ApplicationUpdate:       applicationupdate.New(applicationUpdateMock{}),
		ProductGet:              productget.New(productMock{}),
		ProductList:             productlist.New(productsMock{}),
	})
	for _, tc := range []struct {
		method, path, authorization string
		status                      int
	}{
		{http.MethodGet, "/healthz", "", 204}, {http.MethodGet, "/readyz", "", 204}, {http.MethodGet, "/client/company", "", 401},
		{http.MethodGet, "/client/company", "Bearer test", 200},
		{http.MethodGet, "/client/products", "Bearer test", 200},
		{http.MethodGet, "/client/products/00000000-0000-0000-0000-000000000001", "Bearer test", 200},
		{http.MethodPost, "/client/applications", "Bearer test", 201},
		{http.MethodGet, "/client/applications", "Bearer test", 200},
		{http.MethodGet, "/client/applications/00000000-0000-0000-0000-000000000001", "Bearer test", 200},
		{http.MethodGet, "/client/applications/00000000-0000-0000-0000-000000000001/timeline", "Bearer test", 200},
		{http.MethodGet, "/client/applications/00000000-0000-0000-0000-000000000001/timeline", "", 401},
		{http.MethodGet, "/client/applications/not-a-uuid/timeline", "Bearer test", 400},
		{http.MethodPatch, "/client/applications/00000000-0000-0000-0000-000000000001", "Bearer test", 200},
		{http.MethodPost, "/client/applications/00000000-0000-0000-0000-000000000001/documents", "Bearer test", 201},
		{http.MethodPost, "/client/applications/00000000-0000-0000-0000-000000000001/submit", "Bearer test", 200},
		{http.MethodPost, "/client/applications/00000000-0000-0000-0000-000000000001/offer/accept", "Bearer test", 202},
		{http.MethodPost, "/client/applications/00000000-0000-0000-0000-000000000001/offer/decline", "Bearer test", 202},
		{http.MethodPost, "/client/applications/00000000-0000-0000-0000-000000000001/sign", "Bearer test", 202},
		{http.MethodPost, "/client/applications/00000000-0000-0000-0000-000000000001/cancel", "Bearer test", 202},
		{http.MethodPost, "/client/applications/00000000-0000-0000-0000-000000000001/cancel", "", 401},
	} {
		t.Run(tc.method+tc.path+tc.authorization, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.method == http.MethodPost && strings.HasSuffix(tc.path, "/documents") {
				body := bytes.NewBuffer(nil)
				writer := multipart.NewWriter(body)
				if err := writer.WriteField("document_type", "BANK_STATEMENT"); err != nil {
					t.Fatal(err)
				}
				part, err := writer.CreateFormFile("file", "statement.pdf")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := part.Write([]byte("demo")); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				request = httptest.NewRequest(http.MethodPost, tc.path, body)
				request.Header.Set("Content-Type", writer.FormDataContentType())
				request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000002")
			} else if tc.method == http.MethodPost && tc.path == "/client/applications" {
				request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"product_id":"00000000-0000-0000-0000-000000000001"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000002")
			} else if tc.method == http.MethodPost && strings.HasSuffix(tc.path, "/submit") {
				request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000003")
			} else if tc.method == http.MethodPost && strings.HasSuffix(tc.path, "/offer/accept") {
				request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"offer_id":"00000000-0000-0000-0000-000000000004"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000005")
			} else if tc.method == http.MethodPost && strings.HasSuffix(tc.path, "/offer/decline") {
				request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"offer_id":"00000000-0000-0000-0000-000000000004"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000006")
			} else if tc.method == http.MethodPost && strings.HasSuffix(tc.path, "/sign") {
				request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"offer_id":"00000000-0000-0000-0000-000000000004"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000007")
			} else if tc.method == http.MethodPost && strings.HasSuffix(tc.path, "/cancel") {
				request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000008")
			} else if tc.method == http.MethodPatch {
				request = httptest.NewRequest(http.MethodPatch, tc.path, strings.NewReader(`{"amount":{"amount_kopecks":100,"currency":"RUB"}}`))
				request.Header.Set("Content-Type", "application/merge-patch+json")
				request.Header.Set("If-Match", `"1"`)
			}
			request.Header.Set("Authorization", tc.authorization)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
