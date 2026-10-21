package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	listhandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/list"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/health"
)

var errNotImplemented = errors.New("operation is not implemented")

type server struct {
	health    *health.Handler
	listCases *listhandler.Handler
}

func New(db health.Pinger, readinessTimeout time.Duration, listCases listhandler.UseCase) http.Handler {
	strictHandler := generated.NewStrictHandlerWithOptions(&server{
		health:    health.New(db, readinessTimeout),
		listCases: listhandler.New(listCases),
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
