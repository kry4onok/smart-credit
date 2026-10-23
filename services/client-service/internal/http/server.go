package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	applicationcreate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/create"
	companyget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/get"
	companyupdate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/update"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/health"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	productget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/get"
	productlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/list"
)

var errNotImplemented = errors.New("operation is not implemented")

type server struct {
	health      *health.Handler
	company     *companyget.Handler
	update      *companyupdate.Handler
	application *applicationcreate.Handler
	product     *productget.Handler
	products    *productlist.Handler
}

var _ generated.StrictServerInterface = (*server)(nil)

func New(db health.Pinger, readinessTimeout time.Duration, company *companyget.Handler, update *companyupdate.Handler, application *applicationcreate.Handler, product *productget.Handler, products *productlist.Handler, verifier middleware.Verifier) http.Handler {
	strictHandler := generated.NewStrictHandlerWithOptions(&server{
		health:      health.New(db, readinessTimeout),
		company:     company,
		update:      update,
		application: application,
		product:     product,
		products:    products,
	}, nil, generated.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: responseErrorHandler,
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusBadRequest, "BAD_REQUEST", "Некорректный запрос."))
		},
	})
	handler := generated.Handler(strictHandler)
	protected := middleware.RequireClient(verifier, handler)
	return domainerror.Trace(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			handler.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	}))
}

func (s *server) ClientHealth(ctx context.Context, request generated.ClientHealthRequestObject) (generated.ClientHealthResponseObject, error) {
	return s.health.Health(ctx, request)
}

func (s *server) ClientReady(ctx context.Context, request generated.ClientReadyRequestObject) (generated.ClientReadyResponseObject, error) {
	return s.health.Ready(ctx, request)
}

func responseErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errNotImplemented) {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	domainerror.Write(w, domainerror.Map(r.Context(), err))
}

func (s *server) GetCompany(ctx context.Context, request generated.GetCompanyRequestObject) (generated.GetCompanyResponseObject, error) {
	return s.company.Handle(ctx, request)
}

func (s *server) UpdateCompany(ctx context.Context, request generated.UpdateCompanyRequestObject) (generated.UpdateCompanyResponseObject, error) {
	return s.update.Handle(ctx, request)
}

func (s *server) CreateApplication(ctx context.Context, request generated.CreateApplicationRequestObject) (generated.CreateApplicationResponseObject, error) {
	return s.application.Handle(ctx, request)
}

func (s *server) ListClientProducts(ctx context.Context, request generated.ListClientProductsRequestObject) (generated.ListClientProductsResponseObject, error) {
	return s.products.Handle(ctx, request)
}

func (s *server) GetClientProduct(ctx context.Context, request generated.GetClientProductRequestObject) (generated.GetClientProductResponseObject, error) {
	return s.product.Handle(ctx, request)
}
