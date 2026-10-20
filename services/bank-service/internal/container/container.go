package container

import (
	"context"
	"fmt"
	"sync"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Container struct {
	DB *pgxpool.Pool

	closeOnce sync.Once
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	db, err := postgres.New(ctx, postgres.Config{
		URL:            cfg.Database.URL,
		ConnectTimeout: cfg.Database.ConnectTimeout,
		MaxConnections: cfg.Database.MaxConnections,
		MinConnections: cfg.Database.MinConnections,
	})
	if err != nil {
		return nil, fmt.Errorf("create postgres dependency: %w", err)
	}

	return &Container{DB: db}, nil
}

func (c *Container) Close() {
	if c == nil {
		return
	}

	c.closeOnce.Do(func() {
		if c.DB != nil {
			c.DB.Close()
		}
	})
}
