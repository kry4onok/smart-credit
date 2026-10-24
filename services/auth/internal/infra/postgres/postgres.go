package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, errors.New("invalid AUTH_DATABASE_URL: malformed URL")
		}
		var numErr *strconv.NumError
		if errors.As(err, &numErr) {
			return nil, errors.New("invalid AUTH_DATABASE_URL: invalid numeric parameter")
		}
		return nil, errors.New("invalid AUTH_DATABASE_URL: check connection parameters")
	}
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	poolConfig.MaxConns = cfg.MaxConnections
	poolConfig.MinConns = cfg.MinConnections
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "auth"

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	return pool, nil
}
