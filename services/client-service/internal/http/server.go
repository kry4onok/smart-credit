package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	companyget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/get"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/health"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
)

var errNotImplemented = errors.New("operation is not implemented")

type server struct {
	health  *health.Handler
	company *companyget.Handler
}

var _ generated.StrictServerInterface = (*server)(nil)

func New(db health.Pinger, readinessTimeout time.Duration, company *companyget.Handler, verifier middleware.Verifier) http.Handler {
	strictHandler := generated.NewStrictHandlerWithOptions(&server{
		health:  health.New(db, readinessTimeout),
		company: company,
	}, nil, generated.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: responseErrorHandler,
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
