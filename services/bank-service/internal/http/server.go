package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	gethandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/get"
	listhandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/list"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/health"
)

var errNotImplemented = errors.New("operation is not implemented")

type server struct {
	health    *health.Handler
	listCases *listhandler.Handler
	getCase   *gethandler.Handler
}

func New(
	db health.Pinger,
	readinessTimeout time.Duration,
	listCases listhandler.UseCase,
	getCase gethandler.UseCase,
) http.Handler {
	strictHandler := generated.NewStrictHandlerWithOptions(&server{
		health:    health.New(db, readinessTimeout),
		listCases: listhandler.New(listCases),
		getCase:   gethandler.New(getCase),
	}, nil, generated.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: responseErrorHandler,
	})

	return generated.Handler(strictHandler)
}

func responseErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errNotImplemented) {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	domainerror.Write(w, r, err)
}
