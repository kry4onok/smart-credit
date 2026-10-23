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
	receiverepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/receive"
	scoringrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/scoring"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
	receivecase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	takecase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/take"
	scoringstage "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/scoring"
	enrichmentworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/enrichment"
	scoringworker "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/worker/scoring"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Container struct {
	DB               *pgxpool.Pool
	Transactions     *transaction.Manager
	Scoring          *scoringrepository.Repository
	ScoringWorker    *scoringworker.Worker
	EnrichmentWorker *enrichmentworker.Worker
	ClientEvents     *clientevents.Consumer
	HTTPHandler      http.Handler
	scoringClient    *scoringclient.Client
	bureauClient     *bureauclient.Client

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

	return &Container{
		DB:               db,
		Transactions:     transactions,
		Scoring:          scoringAttempts,
		ScoringWorker:    worker,
		EnrichmentWorker: enrichment,
		ClientEvents:     consumer,
		HTTPHandler:      authmiddleware.DemoAnalyst(httpapi.New(db, cfg.ReadinessTimeout, listCases, getCase, takeCase), demoAnalystID),
		scoringClient:    client,
		bureauClient:     bureau,
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
