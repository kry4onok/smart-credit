package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
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
	companyupdate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/update"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/health"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	productget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/get"
	productlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/list"
)

type server struct {
	health                  *health.Handler
	company                 *companyget.Handler
	update                  *companyupdate.Handler
	application             *applicationcreate.Handler
	applicationCancel       *applicationcancel.Handler
	applicationDocument     *applicationdocument.Handler
	applicationGet          *applicationget.Handler
	applicationList         *applicationlist.Handler
	applicationTimeline     *applicationtimeline.Handler
	applicationOfferAccept  *applicationofferaccept.Handler
	applicationOfferDecline *applicationofferdecline.Handler
	applicationSign         *applicationsign.Handler
	applicationSubmit       *applicationsubmit.Handler
	applicationInfo         *applicationinfo.Handler
	applicationUpdate       *applicationupdate.Handler
	product                 *productget.Handler
	products                *productlist.Handler
}

var _ generated.StrictServerInterface = (*server)(nil)

type Params struct {
	DB                      health.Pinger
	ReadinessTimeout        time.Duration
	Verifier                middleware.Verifier
	CompanyGet              *companyget.Handler
	CompanyUpdate           *companyupdate.Handler
	ApplicationCreate       *applicationcreate.Handler
	ApplicationCancel       *applicationcancel.Handler
	ApplicationDocument     *applicationdocument.Handler
	ApplicationGet          *applicationget.Handler
	ApplicationList         *applicationlist.Handler
	ApplicationTimeline     *applicationtimeline.Handler
	ApplicationOfferAccept  *applicationofferaccept.Handler
	ApplicationOfferDecline *applicationofferdecline.Handler
	ApplicationSign         *applicationsign.Handler
	ApplicationSubmit       *applicationsubmit.Handler
	ApplicationInfo         *applicationinfo.Handler
	ApplicationUpdate       *applicationupdate.Handler
	ProductGet              *productget.Handler
	ProductList             *productlist.Handler
}

func New(params Params) http.Handler {
	strictHandler := generated.NewStrictHandlerWithOptions(&server{
		health:                  health.New(params.DB, params.ReadinessTimeout),
		company:                 params.CompanyGet,
		update:                  params.CompanyUpdate,
		application:             params.ApplicationCreate,
		applicationCancel:       params.ApplicationCancel,
		applicationDocument:     params.ApplicationDocument,
		applicationGet:          params.ApplicationGet,
		applicationList:         params.ApplicationList,
		applicationTimeline:     params.ApplicationTimeline,
		applicationOfferAccept:  params.ApplicationOfferAccept,
		applicationOfferDecline: params.ApplicationOfferDecline,
		applicationSign:         params.ApplicationSign,
		applicationSubmit:       params.ApplicationSubmit,
		applicationInfo:         params.ApplicationInfo,
		applicationUpdate:       params.ApplicationUpdate,
		product:                 params.ProductGet,
		products:                params.ProductList,
	}, nil, generated.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: responseErrorHandler,
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusBadRequest, "BAD_REQUEST", "Некорректный запрос."))
		},
	})
	handler := generated.Handler(strictHandler)
	protected := middleware.RequireClient(params.Verifier, handler)
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

func (s *server) ProvideInfo(ctx context.Context, request generated.ProvideInfoRequestObject) (generated.ProvideInfoResponseObject, error) {
	return s.applicationInfo.Handle(ctx, request)
}

func (s *server) AcceptOffer(ctx context.Context, request generated.AcceptOfferRequestObject) (generated.AcceptOfferResponseObject, error) {
	return s.applicationOfferAccept.Handle(ctx, request)
}

func (s *server) DeclineOffer(ctx context.Context, request generated.DeclineOfferRequestObject) (generated.DeclineOfferResponseObject, error) {
	return s.applicationOfferDecline.Handle(ctx, request)
}

func (s *server) SignContract(ctx context.Context, request generated.SignContractRequestObject) (generated.SignContractResponseObject, error) {
	return s.applicationSign.Handle(ctx, request)
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

func (s *server) GetApplicationTimeline(ctx context.Context, request generated.GetApplicationTimelineRequestObject) (generated.GetApplicationTimelineResponseObject, error) {
	return s.applicationTimeline.Handle(ctx, request)
}

func (s *server) CancelApplication(ctx context.Context, request generated.CancelApplicationRequestObject) (generated.CancelApplicationResponseObject, error) {
	return s.applicationCancel.Handle(ctx, request)
}
