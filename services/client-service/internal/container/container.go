package container

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/config"
	httpapi "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http"
	applicationcreate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/create"
	companyget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/get"
	companyupdate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/update"
	productget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/get"
	productlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/list"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/infra/jwt"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/infra/postgres"
	applicationrepo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/repository/application"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/repository/company"
	productrepo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/repository/product"
	createapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/create"
	getcompany "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/company/get"
	updatecompany "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/company/update"
	getproduct "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/product/get"
	listproducts "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/product/list"
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
		DB: db,
		HTTPHandler: httpapi.New(db, cfg.ReadinessTimeout,
			companyget.New(getcompany.New(company.New(db))),
			companyupdate.New(updatecompany.New(company.New(db))),
			applicationcreate.New(createapplication.New(applicationrepo.New(applicationrepo.Adapt(db)))),
			productget.New(getproduct.New(productrepo.New(db))),
			productlist.New(listproducts.New(productrepo.New(db))),
			jwt.New(cfg.JWT.JWKSURL, cfg.JWT.Issuer, cfg.JWT.Audience, cfg.JWT.Timeout, cfg.JWT.CacheTTL)),
	}, nil
}

func (c *Container) Close() {
	c.DB.Close()
}
