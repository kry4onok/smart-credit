package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/health"
	keyget "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/key/get"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/middleware"
	loginhttp "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/session/login"
	logouthttp "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/session/logout"
	refreshhttp "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/session/refresh"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/sessioncookie"
)

var errNotImplemented = errors.New("operation is not implemented")

type Params struct {
	DB               health.Pinger
	ReadinessTimeout time.Duration
	JWKS             keyget.Usecase
	Login            loginhttp.Usecase
	Refresh          refreshhttp.Usecase
	Logout           logouthttp.Usecase
	Cookie           config.CookieConfig
}

type server struct {
	health  *health.Handler
	keys    *keyget.Handler
	login   *loginhttp.Handler
	refresh *refreshhttp.Handler
	logout  *logouthttp.Handler
}

var _ generated.StrictServerInterface = (*server)(nil)

func New(params Params) http.Handler {
	cookies := sessioncookie.New(params.Cookie)
	strict := generated.NewStrictHandlerWithOptions(&server{
		health:  health.New(params.DB, params.ReadinessTimeout),
		keys:    keyget.New(params.JWKS),
		login:   loginhttp.New(loginhttp.Params{Usecase: params.Login, Cookies: cookies}),
		refresh: refreshhttp.New(params.Refresh),
		logout:  logouthttp.New(logouthttp.Params{Usecase: params.Logout, Cookies: cookies}),
	}, nil, generated.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusBadRequest, "BAD_REQUEST", "Некорректный запрос."))
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, errNotImplemented) {
				domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusNotImplemented, "NOT_IMPLEMENTED", "Операция пока не реализована."))
				return
			}
			domainerror.Write(w, domainerror.Map(r.Context(), err))
		},
	})
	return domainerror.Trace(middleware.NewSessions(params.Cookie).Wrap(generated.Handler(strict)))
}

func (s *server) Login(ctx context.Context, request generated.LoginRequestObject) (generated.LoginResponseObject, error) {
	return s.login.Login(ctx, request)
}

func (s *server) RefreshSession(ctx context.Context, request generated.RefreshSessionRequestObject) (generated.RefreshSessionResponseObject, error) {
	return s.refresh.Refresh(ctx, request)
}

func (s *server) Logout(ctx context.Context, request generated.LogoutRequestObject) (generated.LogoutResponseObject, error) {
	return s.logout.Logout(ctx, request)
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
