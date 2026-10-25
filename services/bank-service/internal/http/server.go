package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	audithandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/audit"
	decidehandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/decide"
	documenthandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/document"
	gethandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/get"
	listhandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/list"
	summaryhandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/summary"
	takehandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/take"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/health"
	productlisthandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/product/list"
)

type server struct {
	health          *health.Handler
	listCases       *listhandler.Handler
	getCase         *gethandler.Handler
	takeCase        *takehandler.Handler
	decideCase      *decidehandler.Handler
	auditCase       *audithandler.Handler
	summaryCase     *summaryhandler.Handler
	listProducts    *productlisthandler.Handler
	getCaseDocument *documenthandler.Handler
}

func New(
	db health.Pinger,
	readinessTimeout time.Duration,
	listCases listhandler.UseCase,
	getCase gethandler.UseCase,
	takeCase takehandler.UseCase,
	decideCase decidehandler.UseCase,
	auditCase audithandler.UseCase,
	summaryCase summaryhandler.UseCase,
	products productlisthandler.Repository,
	getCaseDocument *documenthandler.Handler,
) http.Handler {
	strictHandler := generated.NewStrictHandlerWithOptions(&server{
		health:          health.New(db, readinessTimeout),
		listCases:       listhandler.New(listCases),
		getCase:         gethandler.New(getCase),
		takeCase:        takehandler.New(takeCase),
		decideCase:      decidehandler.New(decideCase),
		auditCase:       audithandler.New(auditCase),
		summaryCase:     summaryhandler.New(summaryCase),
		listProducts:    productlisthandler.New(products),
		getCaseDocument: getCaseDocument,
	}, nil, generated.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: responseErrorHandler,
	})

	return generated.HandlerWithOptions(strictHandler, generated.ChiServerOptions{
		ErrorHandlerFunc: requestParameterErrorHandler,
	})
}

func requestParameterErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	domainerror.Write(w, r, domainerror.BadRequest("INVALID_REQUEST_PARAMETER", err))
}

func responseErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	var domainErr *domainerror.Error
	if !errors.As(err, &domainErr) || domainErr.Status >= http.StatusInternalServerError {
		slog.Error("bank request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	}

	domainerror.Write(w, r, err)
}
