package container

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	httpapi "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/jwt"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/password"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/postgres"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/refreshtoken"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/transaction"
	refreshrepo "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/repository/refresh"
	userrepo "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/repository/user"
	keyget "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/usecase/key/get"
	loginuc "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/usecase/session/login"
	logoutuc "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/usecase/session/logout"
	refreshuc "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/usecase/session/refresh"
	meuc "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/usecase/user/me"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Container struct {
	DB          *pgxpool.Pool
	HTTPHandler http.Handler
	Passwords   *password.Hasher
	Tokens      *jwt.Manager
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	tokens, err := jwt.New(cfg.JWT)
	if err != nil {
		return nil, fmt.Errorf("create JWT dependency: %w", err)
	}
	passwords := password.New()
	// Work-equivalent verification for unknown users; never saved as a credential.
	dummyHash, err := passwords.Hash(uuid.NewString())
	if err != nil {
		return nil, fmt.Errorf("initialize password verifier: %w", err)
	}
	db, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("create postgres dependency: %w", err)
	}
	users, sessions := userrepo.New(db), refreshrepo.New(db)
	transactions, refreshTokens := transaction.New(db), refreshtoken.New()
	login := loginuc.New(loginuc.Params{Users: users, Sessions: sessions, Passwords: passwords,
		AccessTokens: tokens, RefreshTokens: refreshTokens, Transactions: transactions, DummyPasswordHash: dummyHash})
	refresh := refreshuc.New(refreshuc.Params{Sessions: sessions, AccessTokens: tokens,
		RefreshTokens: refreshTokens, Transactions: transactions})
	logout := logoutuc.New(logoutuc.Params{Sessions: sessions, RefreshTokens: refreshTokens})
	return &Container{
		DB: db, Passwords: passwords, Tokens: tokens,
		HTTPHandler: httpapi.New(httpapi.Params{
			DB: db, ReadinessTimeout: cfg.ReadinessTimeout,
			JWKS:  keyget.New(tokens),
			Login: login, Refresh: refresh, Logout: logout, Cookie: cfg.Cookie,
			Me: meuc.New(users), Verifier: tokens,
		}),
	}, nil
}

func (c *Container) Close() { c.DB.Close() }
