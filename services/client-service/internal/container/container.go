package container

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/config"
	httpapi "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Container struct {
	DB          *pgxpool.Pool
	HTTPHandler http.Handler
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	db, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("create postgres dependency: %w", err)
	}
	return &Container{
		DB:          db,
		HTTPHandler: httpapi.New(db, cfg.ReadinessTimeout),
	}, nil
}

func (c *Container) Close() {
	c.DB.Close()
}
