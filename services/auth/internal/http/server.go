package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/health"
	keyget "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/key/get"
)

var errNotImplemented = errors.New("operation is not implemented")

type Params struct {
	DB               health.Pinger
	ReadinessTimeout time.Duration
	JWKS             keyget.Usecase
}

type server struct {
	health *health.Handler
	keys   *keyget.Handler
}

var _ generated.StrictServerInterface = (*server)(nil)

func New(params Params) http.Handler {
	strict := generated.NewStrictHandlerWithOptions(&server{
		health: health.New(params.DB, params.ReadinessTimeout),
		keys:   keyget.New(params.JWKS),
	}, nil, generated.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusBadRequest, "BAD_REQUEST", "Некорректный запрос."))
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, errNotImplemented) {
				domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusNotImplemented, "NOT_IMPLEMENTED", "Операция пока не реализована."))
				return
			}
			domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось обработать запрос."))
		},
	})
	return domainerror.Trace(generated.Handler(strict))
}

func (s *server) AuthHealth(ctx context.Context, request generated.AuthHealthRequestObject) (generated.AuthHealthResponseObject, error) {
	return s.health.Health(ctx, request)
}

func (s *server) AuthReady(ctx context.Context, request generated.AuthReadyRequestObject) (generated.AuthReadyResponseObject, error) {
	return s.health.Ready(ctx, request)
}

func (s *server) GetJwks(ctx context.Context, request generated.GetJwksRequestObject) (generated.GetJwksResponseObject, error) {
	return s.keys.Get(ctx, request)
}
