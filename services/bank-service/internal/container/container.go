package container

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	assistantclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/assistant"
	bureauclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/bureau"
	scoringclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/scoring"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/config"
	clientevents "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/consumer/clientevents"
	httpapi "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http"
	assessmenthandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/assessment"
	documenthandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/document"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	jwtverifier "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/jwt"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/postgres"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	caserepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/case"
	catalogrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/catalog"
	decisionrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/decision"
	disbursementrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/disbursement"
	enrichmentrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/enrichment"
	offerexpiryrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/offerexpiry"
	outboxrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/outbox"
	receiverepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/receive"
	scoringrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/scoring"
	validationrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/validation"
	auditcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/audit"
	decidecase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/decide"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
	receivecase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	summarycase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/summary"
	takecase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/take"
	scoringstage "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/scoring"
	validationstage "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	catalogworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/catalog"
	decisionworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/decision"
	disbursementworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/disbursement"
	enrichmentworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/enrichment"
	offerexpiryworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/offerexpiry"
	outboxworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/outbox"
	scoringworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/scoring"
	validationworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/validation"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Container struct {
	DB                 *pgxpool.Pool
	Transactions       *transaction.Manager
	Scoring            *scoringrepository.Repository
	ScoringWorker      *scoringworker.Worker
	DecisionWorker     *decisionworker.Worker
	DisbursementWorker *disbursementworker.Worker
	OfferExpiryWorker  *offerexpiryworker.Worker
	CatalogWorker      *catalogworker.Worker
	EnrichmentWorker   *enrichmentworker.Worker
	ValidationWorker   *validationworker.Worker
	OutboxWorker       *outboxworker.Worker
	ClientEvents       *clientevents.Consumer
	HTTPHandler        http.Handler
	scoringClient      *scoringclient.Client
	bureauClient       *bureauclient.Client
	assistantClient    *assistantclient.Client
	outboxPublisher    *outboxworker.KafkaPublisher

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
	transactions := transaction.New(db)
	catalogRepository := catalogrepository.New(db, transactions)
	validationCases := validationrepository.New(db)
	validation := validationworker.New(validationstage.New(validationCases, transactions))
	receiveCases := receivecase.New(receiverepository.New(db), transactions)
	brokerAddresses := strings.Split(cfg.KafkaBrokers, ",")
	for index := range brokerAddresses {
		brokerAddresses[index] = strings.TrimSpace(brokerAddresses[index])
	}
	consumer, err := clientevents.New(brokerAddresses, cfg.KafkaGroup, receiveCases)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}
	scoringAttempts := scoringrepository.New(db, transactions)
	enrichmentResults := enrichmentrepository.New(db, transactions)
	takeCase := takecase.New(caseRepository, transactions)
	auditCase := auditcase.New(caseRepository)
	var client *scoringclient.Client
	var worker *scoringworker.Worker
	var bureau *bureauclient.Client
	var assistant *assistantclient.Client
	var enrichment *enrichmentworker.Worker
	if cfg.ScoringTarget != "" {
		client, err = scoringclient.Dial(cfg.ScoringTarget)
		if err != nil {
			consumer.Close()
			db.Close()
			return nil, fmt.Errorf("create scoring dependency: %w", err)
		}
		worker = scoringworker.New(scoringAttempts, scoringstage.New(scoringAttempts, client), scoringAttempts)
	}
	if cfg.BureauTarget != "" {
		bureau, err = bureauclient.Dial(cfg.BureauTarget)
		if err != nil {
			consumer.Close()
			if client != nil {
				_ = client.Close()
			}
			db.Close()
			return nil, fmt.Errorf("create bureau dependency: %w", err)
		}
		enrichment = enrichmentworker.New(enrichmentResults, bureau)
	}
	if cfg.AssistantTarget != "" {
		assistant, err = assistantclient.Dial(cfg.AssistantTarget, cfg.AssistantTimeout)
		if err != nil {
			consumer.Close()
			if client != nil {
				_ = client.Close()
			}
			if bureau != nil {
				_ = bureau.Close()
			}
			db.Close()
			return nil, fmt.Errorf("create assistant dependency: %w", err)
		}
	}
	publisher, err := outboxworker.NewKafkaPublisher(brokerAddresses)
	if err != nil {
		consumer.Close()
		if client != nil {
			_ = client.Close()
		}
		if bureau != nil {
			_ = bureau.Close()
		}
		if assistant != nil {
			_ = assistant.Close()
		}
		db.Close()
		return nil, fmt.Errorf("create bank outbox publisher: %w", err)
	}

	var explainer decidecase.Assistant
	if assistant != nil {
		explainer = assistant
	}
	decideCase := decidecase.New(caseRepository, transactions, explainer)
	var summarizer summarycase.Assistant
	if assistant != nil {
		summarizer = assistant
	}
	return &Container{
		DB:                 db,
		Transactions:       transactions,
		Scoring:            scoringAttempts,
		ScoringWorker:      worker,
		DecisionWorker:     decisionworker.New(decisionrepository.New(db, transactions)),
		DisbursementWorker: disbursementworker.New(disbursementrepository.New(db, transactions)),
		OfferExpiryWorker:  offerexpiryworker.New(offerexpiryrepository.New(db, transactions)),
		CatalogWorker:      catalogworker.New(catalogRepository),
		EnrichmentWorker:   enrichment,
		ValidationWorker:   validation,
		OutboxWorker:       outboxworker.New(outboxrepository.New(db), publisher),
		ClientEvents:       consumer,
		HTTPHandler:        authmiddleware.RequireEmployee(jwtverifier.New(cfg.JWT), assessmenthandler.New(db, transactions, cfg.AssistantTarget).Wrap(httpapi.New(db, cfg.ReadinessTimeout, listCases, getCase, takeCase, decideCase, auditCase, summarycase.New(caseRepository, summarizer), catalogRepository, documenthandler.New(getCase, cfg.ClientDocumentURL, cfg.ClientDocumentTimeout)))),
		scoringClient:      client,
		bureauClient:       bureau,
		assistantClient:    assistant,
		outboxPublisher:    publisher,
	}, nil
}

func (c *Container) Close() {
	if c == nil {
		return
	}

	c.closeOnce.Do(func() {
		if c.ClientEvents != nil {
			c.ClientEvents.Close()
		}
		if c.outboxPublisher != nil {
			c.outboxPublisher.Close()
		}
		if c.scoringClient != nil {
			_ = c.scoringClient.Close()
		}
		if c.bureauClient != nil {
			_ = c.bureauClient.Close()
		}
		if c.assistantClient != nil {
			_ = c.assistantClient.Close()
		}
		if c.DB != nil {
			c.DB.Close()
		}
	})
}
