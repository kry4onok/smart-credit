// Command dev-seed installs local demo accounts after auth migrations.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/devseed"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	reset := os.Getenv("AUTH_DEV_SEED_RESET_PASSWORDS")
	if reset != "" && reset != "1" {
		return fmt.Errorf("AUTH_DEV_SEED_RESET_PASSWORDS must be 1 when set")
	}
	credentials := devseed.Credentials{
		AnalystPassword: os.Getenv("AUTH_DEV_ANALYST_PASSWORD"),
		AdminPassword:   os.Getenv("AUTH_DEV_ADMIN_PASSWORD"),
		ClientPassword:  os.Getenv("AUTH_DEV_CLIENT_PASSWORD"),
		ResetPasswords:  reset == "1",
	}
	if err := credentials.Validate(); err != nil {
		return err
	}
	databaseURL := os.Getenv("AUTH_DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("AUTH_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("invalid AUTH_DATABASE_URL")
	}
	defer db.Close()
	accounts, err := devseed.Run(ctx, db, credentials)
	if err != nil {
		return err
	}
	for _, account := range accounts {
		fmt.Printf("%s %s %s\n", account.Role, account.Email, account.ID)
	}
	return nil
}
