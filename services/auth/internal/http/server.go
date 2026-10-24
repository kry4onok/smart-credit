package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
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
	mehttp "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/user/me"
	"github.com/go-chi/chi/v5"
)

type Params struct {
	DB               health.Pinger
	ReadinessTimeout time.Duration
	JWKS             keyget.Usecase
	Login            loginhttp.Usecase
	Refresh          refreshhttp.Usecase
	Logout           logouthttp.Usecase
	Cookie           config.CookieConfig
	Me               mehttp.Usecase
	Verifier         middleware.Verifier
	Logger           *slog.Logger
}

type server struct {
	health  *health.Handler
	keys    *keyget.Handler
	login   *loginhttp.Handler
	refresh *refreshhttp.Handler
	logout  *logouthttp.Handler
	me      *mehttp.Handler
}

var _ generated.StrictServerInterface = (*server)(nil)

func New(params Params) http.Handler {
	router := chi.NewRouter()
	router.Use(domainerror.Trace, middleware.Requests(params.Logger), middleware.Recover(params.Logger), middleware.NewSessions(params.Cookie).Wrap)
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusNotFound, "NOT_FOUND", "Ресурс не найден."))
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		var allowed []string
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace} {
			if router.Match(chi.NewRouteContext(), method, r.URL.Path) {
				allowed = append(allowed, method)
			}
		}
		if len(allowed) != 0 {
			w.Header().Set("Allow", strings.Join(allowed, ", "))
		}
		domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Метод не поддерживается."))
	})
	cookies := sessioncookie.New(params.Cookie)
	strict := generated.NewStrictHandlerWithOptions(&server{
		health:  health.New(params.DB, params.ReadinessTimeout),
		keys:    keyget.New(params.JWKS),
		login:   loginhttp.New(loginhttp.Params{Usecase: params.Login, Cookies: cookies}),
		refresh: refreshhttp.New(params.Refresh),
		logout:  logouthttp.New(logouthttp.Params{Usecase: params.Logout, Cookies: cookies}),
		me:      mehttp.New(params.Me),
	}, []generated.StrictMiddlewareFunc{middleware.RequireBearer(params.Verifier)}, generated.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusBadRequest, "BAD_REQUEST", "Некорректный запрос."))
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			problem := domainerror.Map(r.Context(), err)
			if r.URL.Path == "/auth/me" && problem.Status == http.StatusUnauthorized {
				w.Header().Set("WWW-Authenticate", "Bearer")
			}
			domainerror.Write(w, problem)
		},
	})
	return generated.HandlerWithOptions(strict, generated.ChiServerOptions{BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusBadRequest, "BAD_REQUEST", "Некорректный запрос."))
		}})
}

func (s *server) GetCurrentUser(ctx context.Context, request generated.GetCurrentUserRequestObject) (generated.GetCurrentUserResponseObject, error) {
	return s.me.Me(ctx, request)
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
