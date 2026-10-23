package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/container"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("bank-service stopped with error", "error", err)
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

	server := httpserver.New(httpserver.Config{
		Address:           cfg.HTTP.Address,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}, dependencies.HTTPHandler)
	serverErrors := make(chan error, 1)
	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	var workers sync.WaitGroup
	go func() {
		serverErrors <- server.Run()
	}()
	if dependencies.ScoringWorker != nil {
		workers.Add(1)
		go func() { defer workers.Done(); dependencies.ScoringWorker.Run(workerCtx, logger) }()
	}
	if dependencies.EnrichmentWorker != nil {
		workers.Add(1)
		go func() { defer workers.Done(); dependencies.EnrichmentWorker.Run(workerCtx, logger) }()
	}
	workers.Add(1)
	go func() { defer workers.Done(); dependencies.ValidationWorker.Run(workerCtx, logger) }()
	workers.Add(1)
	go func() { defer workers.Done(); dependencies.OutboxWorker.Run(workerCtx, logger) }()
	workers.Add(1)
	go func() { defer workers.Done(); dependencies.ClientEvents.Run(workerCtx, logger) }()

	logger.Info("bank-service started", "address", cfg.HTTP.Address)

	select {
	case err := <-serverErrors:
		stopWorkers()
		workers.Wait()
		return err
	case <-ctx.Done():
		logger.Info("bank-service shutdown started")
	}
	stopWorkers()
	workers.Wait()

	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		closeErr := server.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}

	if err := <-serverErrors; err != nil {
		return err
	}

	logger.Info("bank-service stopped")
	return nil
}
