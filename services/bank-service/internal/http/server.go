package httpapi

import (
	"errors"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/health"
)

var errNotImplemented = errors.New("operation is not implemented")

type server struct {
	health *health.Handler
}

func New(db health.Pinger) http.Handler {
	strictHandler := generated.NewStrictHandlerWithOptions(&server{
		health: health.New(db),
	}, nil, generated.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: responseErrorHandler,
	})

	return generated.Handler(strictHandler)
}

func responseErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	if errors.Is(err, errNotImplemented) {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
}
