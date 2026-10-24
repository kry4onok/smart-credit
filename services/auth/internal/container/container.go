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
	keyget "github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/usecase/key/get"
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
	db, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("create postgres dependency: %w", err)
	}
	return &Container{
		DB: db, Passwords: password.New(), Tokens: tokens,
		HTTPHandler: httpapi.New(httpapi.Params{
			DB: db, ReadinessTimeout: cfg.ReadinessTimeout,
			JWKS: keyget.New(tokens),
		}),
	}, nil
}

func (c *Container) Close() { c.DB.Close() }
