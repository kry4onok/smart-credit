package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	scoringclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/scoring"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	scoringrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/scoring"
	stage "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/scoring"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	caseIDText := flag.String("case-id", "", "UUID of a case with successful bureau responses")
	target := flag.String("scoring-target", "127.0.0.1:50051", "scoring gRPC address")
	flag.Parse()
	if err := run(*caseIDText, *target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(caseIDText, target string) error {
	caseID, err := uuid.Parse(caseIDText)
	if err != nil || caseID == uuid.Nil {
		return fmt.Errorf("-case-id must be a nonzero UUID")
	}
	url := os.Getenv("BANK_DATABASE_URL")
	if url == "" {
		return fmt.Errorf("BANK_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		return fmt.Errorf("open bank_db: %w", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping bank_db: %w", err)
	}
	client, err := scoringclient.Dial(target)
	if err != nil {
		return fmt.Errorf("connect to scoring: %w", err)
	}
	defer func() { _ = client.Close() }()
	runner := stage.New(scoringrepository.New(db, transaction.New(db)), client)
	attemptID, applied, err := runner.Run(ctx, caseID)
	if err != nil {
		return err
	}
	fmt.Printf("scoring attempt %s completed; current=%t\n", attemptID, applied)
	return nil
}
