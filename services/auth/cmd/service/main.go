package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/container"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, logger); err != nil {
		logger.Error("auth stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	dependencies, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	defer dependencies.Close()

	server := httpserver.New(cfg.HTTP, dependencies.HTTPHandler)
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Run() }()
	logger.Info("auth started", "address", cfg.HTTP.Address)
	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		logger.Info("auth shutdown started")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serverErrors; err != nil {
		return err
	}
	logger.Info("auth stopped")
	return nil
}
