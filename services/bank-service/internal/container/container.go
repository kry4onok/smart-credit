package container

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/config"
	httpapi "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/postgres"
	caserepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/case"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Container struct {
	DB          *pgxpool.Pool
	HTTPHandler http.Handler

	closeOnce sync.Once
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	db, err := postgres.New(ctx, postgres.Config{
		URL:               cfg.Database.URL,
		ConnectTimeout:    cfg.Database.ConnectTimeout,
		MaxConnections:    cfg.Database.MaxConnections,
		MinConnections:    cfg.Database.MinConnections,
		MaxConnLifetime:   cfg.Database.MaxConnLifetime,
		MaxConnIdleTime:   cfg.Database.MaxConnIdleTime,
		HealthCheckPeriod: cfg.Database.HealthCheckPeriod,
	})
	if err != nil {
		return nil, fmt.Errorf("create postgres dependency: %w", err)
	}
	caseRepository := caserepository.New(db)
	listCases := listcase.New(caseRepository)
	getCase := getcase.New(caseRepository)

	return &Container{
		DB:          db,
		HTTPHandler: httpapi.New(db, cfg.ReadinessTimeout, listCases, getCase),
	}, nil
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
