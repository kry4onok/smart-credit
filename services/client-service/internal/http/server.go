package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	applicationcreate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/create"
	applicationdocument "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/document"
	applicationget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/get"
	applicationlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/list"
	applicationsubmit "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/submit"
	applicationupdate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/update"
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
	health              *health.Handler
	company             *companyget.Handler
	update              *companyupdate.Handler
	application         *applicationcreate.Handler
	applicationDocument *applicationdocument.Handler
	applicationGet      *applicationget.Handler
	applicationList     *applicationlist.Handler
	applicationSubmit   *applicationsubmit.Handler
	applicationUpdate   *applicationupdate.Handler
	product             *productget.Handler
	products            *productlist.Handler
}

var _ generated.StrictServerInterface = (*server)(nil)

func New(db health.Pinger, readinessTimeout time.Duration, company *companyget.Handler, update *companyupdate.Handler, application *applicationcreate.Handler, applicationDocument *applicationdocument.Handler, applicationGet *applicationget.Handler, applicationList *applicationlist.Handler, applicationSubmit *applicationsubmit.Handler, applicationUpdate *applicationupdate.Handler, product *productget.Handler, products *productlist.Handler, verifier middleware.Verifier) http.Handler {
	strictHandler := generated.NewStrictHandlerWithOptions(&server{
		health:              health.New(db, readinessTimeout),
		company:             company,
		update:              update,
		application:         application,
		applicationDocument: applicationDocument,
		applicationGet:      applicationGet,
		applicationList:     applicationList,
		applicationSubmit:   applicationSubmit,
		applicationUpdate:   applicationUpdate,
		product:             product,
		products:            products,
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

func (s *server) AddApplicationDocument(ctx context.Context, request generated.AddApplicationDocumentRequestObject) (generated.AddApplicationDocumentResponseObject, error) {
	return s.applicationDocument.Handle(ctx, request)
}

func (s *server) GetApplication(ctx context.Context, request generated.GetApplicationRequestObject) (generated.GetApplicationResponseObject, error) {
	return s.applicationGet.Handle(ctx, request)
}

func (s *server) ListApplications(ctx context.Context, request generated.ListApplicationsRequestObject) (generated.ListApplicationsResponseObject, error) {
	return s.applicationList.Handle(ctx, request)
}

func (s *server) SubmitApplication(ctx context.Context, request generated.SubmitApplicationRequestObject) (generated.SubmitApplicationResponseObject, error) {
	return s.applicationSubmit.Handle(ctx, request)
}

func (s *server) UpdateApplication(ctx context.Context, request generated.UpdateApplicationRequestObject) (generated.UpdateApplicationResponseObject, error) {
	return s.applicationUpdate.Handle(ctx, request)
}

func (s *server) ListClientProducts(ctx context.Context, request generated.ListClientProductsRequestObject) (generated.ListClientProductsResponseObject, error) {
	return s.products.Handle(ctx, request)
}

func (s *server) GetClientProduct(ctx context.Context, request generated.GetClientProductRequestObject) (generated.GetClientProductResponseObject, error) {
	return s.product.Handle(ctx, request)
}
