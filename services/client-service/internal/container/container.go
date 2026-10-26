package container

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/config"
	bankevents "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/consumer/bankevents"
	httpapi "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http"
	applicationcancel "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/cancel"
	applicationcreate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/create"
	applicationdocument "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/document"
	documentcontent "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/document/content"
	applicationget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/get"
	applicationinfo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/info"
	applicationlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/list"
	applicationofferaccept "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/offer/accept"
	applicationofferdecline "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/offer/decline"
	applicationsign "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/sign"
	applicationsubmit "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/submit"
	applicationtimeline "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/timeline"
	applicationupdate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/update"
	assessmenthandler "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/assessment"
	companyget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/get"
	companyupdate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/update"
	productget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/get"
	productlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/list"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/infra/jwt"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/infra/postgres"
	applicationrepo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/repository/application"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/repository/company"
	productrepo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/repository/product"
	cancelapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/cancel"
	createapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/create"
	addapplicationdocument "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/document"
	getdocumentcontent "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/document/content"
	getapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/get"
	provideinfo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/info"
	listapplications "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
	acceptoffer "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/offer/accept"
	declineoffer "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/offer/decline"
	signcontract "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/sign"
	submitapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/submit"
	gettimeline "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/timeline"
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
		HTTPHandler: httpapi.New(httpapi.Params{
			DB:                      db,
			Assessment:              assessmenthandler.New(db, cfg.AssistantTarget),
			ReadinessTimeout:        cfg.ReadinessTimeout,
			Verifier:                jwt.New(cfg.JWT),
			CompanyGet:              companyget.New(getcompany.New(company.New(db))),
			CompanyUpdate:           companyupdate.New(updatecompany.New(company.New(db))),
			ApplicationCreate:       applicationcreate.New(createapplication.New(applicationRepository)),
			ApplicationCancel:       applicationcancel.New(cancelapplication.New(applicationRepository)),
			ApplicationDocument:     applicationdocument.New(addapplicationdocument.New(applicationRepository)),
			DocumentContent:         documentcontent.New(getdocumentcontent.New(applicationRepository)),
			ApplicationGet:          applicationget.New(getapplication.New(applicationRepository)),
			ApplicationList:         applicationlist.New(listapplications.New(applicationRepository)),
			ApplicationTimeline:     applicationtimeline.New(gettimeline.New(applicationRepository)),
			ApplicationOfferAccept:  applicationofferaccept.New(acceptoffer.New(applicationRepository)),
			ApplicationOfferDecline: applicationofferdecline.New(declineoffer.New(applicationRepository)),
			ApplicationSign:         applicationsign.New(signcontract.New(applicationRepository)),
			ApplicationSubmit:       applicationsubmit.New(submitapplication.New(applicationRepository)),
			ApplicationInfo:         applicationinfo.New(provideinfo.New(applicationRepository)),
			ApplicationUpdate:       applicationupdate.New(updateapplication.New(applicationRepository)),
			ProductGet:              productget.New(getproduct.New(productrepo.New(db))),
			ProductList:             productlist.New(listproducts.New(productrepo.New(db))),
		}),
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
