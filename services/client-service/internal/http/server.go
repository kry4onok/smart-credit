package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/health"
)

var errNotImplemented = errors.New("operation is not implemented")

type server struct {
	health *health.Handler
}

var _ generated.StrictServerInterface = (*server)(nil)

func New(db health.Pinger, readinessTimeout time.Duration) http.Handler {
	strictHandler := generated.NewStrictHandlerWithOptions(&server{
		health: health.New(db, readinessTimeout),
	}, nil, generated.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: responseErrorHandler,
	})
	return generated.Handler(strictHandler)
}

func (s *server) ClientHealth(ctx context.Context, request generated.ClientHealthRequestObject) (generated.ClientHealthResponseObject, error) {
	return s.health.Health(ctx, request)
}

func (s *server) ClientReady(ctx context.Context, request generated.ClientReadyRequestObject) (generated.ClientReadyResponseObject, error) {
	return s.health.Ready(ctx, request)
}

func responseErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	if errors.Is(err, errNotImplemented) {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}
