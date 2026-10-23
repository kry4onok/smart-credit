package container

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/config"
	bankevents "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/consumer/bankevents"
	httpapi "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http"
	applicationcreate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/create"
	applicationdocument "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/document"
	applicationget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/get"
	applicationinfo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/info"
	applicationlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/list"
	applicationsubmit "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/submit"
	applicationupdate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/update"
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
	addapplicationdocument "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/document"
	getapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/get"
	provideinfo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/info"
	listapplications "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
	submitapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/submit"
	updateapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/update"
	getcompany "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/company/get"
	updatecompany "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/company/update"
	getproduct "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/product/get"
	listproducts "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/product/list"
	clientoutbox "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/worker/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Container struct {
	DB          *pgxpool.Pool
	HTTPHandler http.Handler
	Outbox      *clientoutbox.Worker
	BankEvents  *bankevents.Consumer
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	db, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("create postgres dependency: %w", err)
	}
	applicationRepository := applicationrepo.New(applicationrepo.Adapt(db), cfg.DocumentStoragePath)
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	for index := range brokers {
		brokers[index] = strings.TrimSpace(brokers[index])
	}
	outbox, err := clientoutbox.New(db, brokers)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create client outbox relay: %w", err)
	}
	bankEvents, err := bankevents.New(brokers, cfg.KafkaConsumerGroup, bankevents.NewProjection(db))
	if err != nil {
		outbox.Close()
		db.Close()
		return nil, fmt.Errorf("create bank events consumer: %w", err)
	}
	return &Container{
		DB:         db,
		Outbox:     outbox,
		BankEvents: bankEvents,
		HTTPHandler: httpapi.New(db, cfg.ReadinessTimeout,
			companyget.New(getcompany.New(company.New(db))),
			companyupdate.New(updatecompany.New(company.New(db))),
			applicationcreate.New(createapplication.New(applicationRepository)),
			applicationdocument.New(addapplicationdocument.New(applicationRepository)),
			applicationget.New(getapplication.New(applicationRepository)),
			applicationlist.New(listapplications.New(applicationRepository)),
			applicationsubmit.New(submitapplication.New(applicationRepository)),
			applicationinfo.New(provideinfo.New(applicationRepository)),
			applicationupdate.New(updateapplication.New(applicationRepository)),
			productget.New(getproduct.New(productrepo.New(db))),
			productlist.New(listproducts.New(productrepo.New(db))),
			jwt.New(cfg.JWT.JWKSURL, cfg.JWT.Issuer, cfg.JWT.Audience, cfg.JWT.Timeout, cfg.JWT.CacheTTL)),
	}, nil
}

func (c *Container) Close() {
	if c.Outbox != nil {
		c.Outbox.Close()
	}
	if c.BankEvents != nil {
		c.BankEvents.Close()
	}
	c.DB.Close()
}
