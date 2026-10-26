package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	documenthandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/document"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/jwt"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	summarycase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/summary"
	"github.com/google/uuid"
)

type pingerStub struct {
	err   error
	calls int
}

type summaryUseCaseStub struct {
	output summarycase.Output
	err    error
}

type productRepositoryStub struct {
	products []*commonpb.Product
	calls    int
}

type documentCaseStub struct{ value model.Case }

func (s documentCaseStub) Execute(context.Context, getcase.Input) (getcase.Output, error) {
	return getcase.Output{Case: s.value}, nil
}

func (s *productRepositoryStub) ListActive(context.Context) ([]*commonpb.Product, error) {
	s.calls++
	return s.products, nil
}

type verifierStub func(context.Context, string) (jwt.Principal, error)

func (f verifierStub) Verify(ctx context.Context, token string) (jwt.Principal, error) {
	return f(ctx, token)
}

func (s summaryUseCaseStub) Execute(context.Context, uuid.UUID) (summarycase.Output, error) {
	return s.output, s.err
}

func (p *pingerStub) Ping(context.Context) error {
	p.calls++
	return p.err
}

func TestHealth(t *testing.T) {
	db := &pingerStub{err: errors.New("database is unavailable")}
	handler := New(db, time.Second, nil, nil, nil, nil, nil, nil, nil, nil)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if db.calls != 0 {
		t.Fatalf("database ping calls = %d, want 0", db.calls)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name       string
		pingError  error
		wantStatus int
	}{
		{name: "database is ready", wantStatus: http.StatusNoContent},
		{name: "database is unavailable", pingError: errors.New("connection refused"), wantStatus: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &pingerStub{err: tt.pingError}
			handler := New(db, time.Second, nil, nil, nil, nil, nil, nil, nil, nil)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil))

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if db.calls != 1 {
				t.Fatalf("database ping calls = %d, want 1", db.calls)
			}
		})
	}
}

func TestSummaryRouteRequiresEmployee(t *testing.T) {
	handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil, nil, nil, nil)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/backoffice/applications/00000000-0000-0000-0000-000000000001/summary", nil))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestSummaryRouteReturnsTextAndSource(t *testing.T) {
	handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil,
		summaryUseCaseStub{output: summarycase.Output{Text: "Продукт: кредит.", Source: "template"}}, nil, nil)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/backoffice/applications/10000000-0000-4000-8000-000000000001/summary", nil)
	request = request.WithContext(authmiddleware.WithAnalystID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Text   string `json:"text"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Text != "Продукт: кредит." || body.Source != "template" {
		t.Fatalf("body = %+v, err = %v", body, err)
	}
}

func TestSummaryRouteReturnsNotFound(t *testing.T) {
	handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil,
		summaryUseCaseStub{err: summarycase.ErrNotFound}, nil, nil)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/backoffice/applications/10000000-0000-4000-8000-000000000001/summary", nil)
	request = request.WithContext(authmiddleware.WithAnalystID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestAuditRouteRequiresEmployee(t *testing.T) {
	handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil, nil, nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/backoffice/applications/10000000-0000-4000-8000-000000000001/audit", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}

func TestTakeInvalidHeadersReturnProblemJSON(t *testing.T) {
	tests := []struct {
		name           string
		idempotencyKey string
		ifMatch        string
	}{
		{
			name:           "missing If-Match",
			idempotencyKey: "11111111-1111-4111-8111-111111111111",
		},
		{
			name:           "invalid Idempotency-Key",
			idempotencyKey: "not-a-uuid",
			ifMatch:        `"1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil, nil, nil, nil)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
				"/backoffice/applications/10000000-0000-4000-8000-000000000001/take", nil)
			request.Header.Set("Idempotency-Key", tt.idempotencyKey)
			if tt.ifMatch != "" {
				request.Header.Set("If-Match", tt.ifMatch)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "application/problem+json" {
				t.Fatalf("Content-Type = %q, want application/problem+json", contentType)
			}
			var problem common.Problem
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if problem.Status != http.StatusBadRequest || problem.Code != "INVALID_REQUEST_PARAMETER" ||
				problem.Detail == nil || problem.TraceId == "" {
				t.Fatalf("problem = %+v", problem)
			}
		})
	}
}

func TestProductsRouteReturnsActiveTermsOnlyToEmployees(t *testing.T) {
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	repository := &productRepositoryStub{products: []*commonpb.Product{{
		ProductId: id.String(), Version: 2, Code: "WORKING", Name: "Оборотный кредит",
		Description:           "Пополнение оборотных средств",
		PurposeTags:           []commonpb.PurposeTag{commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL},
		RequiredDocumentTypes: []commonpb.DocumentType{commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT},
		MinAmount:             &commonpb.Money{AmountKopecks: 100000000, Currency: "RUB"},
		MaxAmount:             &commonpb.Money{AmountKopecks: 500000000, Currency: "RUB"},
		MinTermMonths:         6, MaxTermMonths: 24, IndicativeRateBps: 1650,
	}}}
	api := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil, nil, repository, nil)
	handler := authmiddleware.RequireEmployee(verifierStub(func(_ context.Context, token string) (jwt.Principal, error) {
		return jwt.Principal{UserID: uuid.New(), Role: token}, nil
	}), api)
	for _, test := range []struct {
		role   string
		status int
	}{
		{"", http.StatusUnauthorized},
		{"CLIENT", http.StatusForbidden},
		{"ANALYST", http.StatusOK},
		{"ADMIN", http.StatusOK},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/backoffice/products", nil)
		if test.role != "" {
			request.Header.Set("Authorization", "Bearer "+test.role)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("role %q: status = %d, body = %s", test.role, response.Code, response.Body.String())
		}
		if test.status != http.StatusOK {
			continue
		}
		var body struct {
			Items []common.Product `json:"items"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Items) != 1 || body.Items[0].Id != id || body.Items[0].Version != 2 ||
			body.Items[0].Name != "Оборотный кредит" || body.Items[0].MaxAmount.AmountKopecks != 500000000 ||
			body.Items[0].IndicativeRateBps == nil || *body.Items[0].IndicativeRateBps != 1650 ||
			len(body.Items[0].PurposeTags) != 1 || body.Items[0].PurposeTags[0] != common.PurposeTagWORKINGCAPITAL ||
			len(body.Items[0].RequiredDocumentTypes) != 1 || body.Items[0].RequiredDocumentTypes[0] != common.BANKSTATEMENT {
			t.Fatalf("role %q: items = %+v", test.role, body.Items)
		}
	}
	if repository.calls != 2 {
		t.Fatalf("repository calls = %d, want 2", repository.calls)
	}
}

func TestCaseDocumentDownloadChecksRoleMembershipAndChecksum(t *testing.T) {
	applicationID := uuid.New()
	documentID := uuid.New()
	content := []byte("demo statement")
	checksum := sha256.Sum256(content)
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if r.URL.Path != "/internal/client/applications/"+applicationID.String()+"/documents/"+documentID.String()+"/content" ||
			r.Header.Get("Authorization") != "Bearer ANALYST" {
			t.Errorf("upstream request = %s, authorization = %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write(content)
	}))
	defer upstream.Close()
	caseReader := documentCaseStub{value: model.Case{Documents: []model.DocumentMetadata{{
		ID: documentID, FileName: "example.txt", SHA256: hex.EncodeToString(checksum[:]),
	}}}}
	api := New(&pingerStub{}, time.Second, nil, caseReader, nil, nil, nil, nil, nil,
		documenthandler.New(caseReader, upstream.URL, time.Second))
	handler := authmiddleware.RequireEmployee(verifierStub(func(_ context.Context, token string) (jwt.Principal, error) {
		return jwt.Principal{UserID: uuid.New(), Role: token}, nil
	}), api)
	call := func(role string, id uuid.UUID) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/backoffice/applications/"+applicationID.String()+"/documents/"+id.String()+"/content", nil)
		request.Header.Set("Authorization", "Bearer "+role)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	if got := call("CLIENT", documentID); got.Code != http.StatusForbidden {
		t.Fatalf("client status = %d", got.Code)
	}
	if got := call("ANALYST", uuid.New()); got.Code != http.StatusNotFound {
		t.Fatalf("unknown document status = %d", got.Code)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream calls before allowed document = %d", upstreamCalls)
	}
	response := call("ANALYST", documentID)
	if response.Code != http.StatusOK || response.Body.String() != string(content) ||
		response.Header().Get("Content-Disposition") != `attachment; filename=example.txt` {
		t.Fatalf("download status = %d, headers = %v, body = %q", response.Code, response.Header(), response.Body.String())
	}
	caseReader.value.Documents[0].SHA256 = strings.Repeat("0", 64)
	badAPI := New(&pingerStub{}, time.Second, nil, caseReader, nil, nil, nil, nil, nil,
		documenthandler.New(caseReader, upstream.URL, time.Second))
	badHandler := authmiddleware.RequireEmployee(verifierStub(func(_ context.Context, token string) (jwt.Principal, error) {
		return jwt.Principal{UserID: uuid.New(), Role: token}, nil
	}), badAPI)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/backoffice/applications/"+applicationID.String()+"/documents/"+documentID.String()+"/content", nil)
	request.Header.Set("Authorization", "Bearer ANALYST")
	badResponse := httptest.NewRecorder()
	badHandler.ServeHTTP(badResponse, request)
	if badResponse.Code != http.StatusBadGateway || badResponse.Body.String() == string(content) {
		t.Fatalf("checksum mismatch status = %d, body = %q", badResponse.Code, badResponse.Body.String())
	}
}
