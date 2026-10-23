package container

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	bureauclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/bureau"
	scoringclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/scoring"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/config"
	clientevents "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/consumer/clientevents"
	httpapi "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/postgres"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	caserepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/case"
	enrichmentrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/enrichment"
	outboxrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/outbox"
	receiverepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/receive"
	scoringrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/scoring"
	validationrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/validation"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
	receivecase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	takecase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/take"
	scoringstage "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/scoring"
	validationstage "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	enrichmentworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/enrichment"
	outboxworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/outbox"
	scoringworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/scoring"
	validationworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Container struct {
	DB               *pgxpool.Pool
	Transactions     *transaction.Manager
	Scoring          *scoringrepository.Repository
	ScoringWorker    *scoringworker.Worker
	EnrichmentWorker *enrichmentworker.Worker
	ValidationWorker *validationworker.Worker
	OutboxWorker     *outboxworker.Worker
	ClientEvents     *clientevents.Consumer
	HTTPHandler      http.Handler
	scoringClient    *scoringclient.Client
	bureauClient     *bureauclient.Client
	outboxPublisher  *outboxworker.KafkaPublisher

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
	demoAnalystID := uuid.Nil
	if cfg.DemoAnalystID != "" {
		demoAnalystID, err = uuid.Parse(cfg.DemoAnalystID)
		if err != nil || demoAnalystID == uuid.Nil {
			consumer.Close()
			db.Close()
			return nil, fmt.Errorf("invalid BANK_DEMO_ANALYST_ID")
		}
	}
	var client *scoringclient.Client
	var worker *scoringworker.Worker
	var bureau *bureauclient.Client
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
	publisher, err := outboxworker.NewKafkaPublisher(brokerAddresses)
	if err != nil {
		consumer.Close()
		if client != nil {
			_ = client.Close()
		}
		if bureau != nil {
			_ = bureau.Close()
		}
		db.Close()
		return nil, fmt.Errorf("create bank outbox publisher: %w", err)
	}

	return &Container{
		DB:               db,
		Transactions:     transactions,
		Scoring:          scoringAttempts,
		ScoringWorker:    worker,
		EnrichmentWorker: enrichment,
		ValidationWorker: validation,
		OutboxWorker:     outboxworker.New(outboxrepository.New(db), publisher),
		ClientEvents:     consumer,
		HTTPHandler:      authmiddleware.DemoAnalyst(httpapi.New(db, cfg.ReadinessTimeout, listCases, getCase, takeCase), demoAnalystID),
		scoringClient:    client,
		bureauClient:     bureau,
		outboxPublisher:  publisher,
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
		if c.DB != nil {
			c.DB.Close()
		}
	})
}
