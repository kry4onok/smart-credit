package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bureau-mocks/internal/grpcserver"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bureau-mocks/internal/persona"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) > 1 {
		if len(os.Args) != 2 || os.Args[1] != "healthcheck" {
			logger.Error("unknown bureau-mocks command")
			os.Exit(2)
		}
		if err := checkHealth(); err != nil {
			logger.Error("bureau-mocks healthcheck failed", "error", err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, logger); err != nil {
		logger.Error("bureau-mocks stopped with error", "error", err)
		os.Exit(1)
	}
}

func checkHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	connection, err := grpc.NewClient("127.0.0.1:50053", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	defer func() { _ = connection.Close() }()
	response, err := healthpb.NewHealthClient(connection).Check(ctx, &healthpb.HealthCheckRequest{
		Service: bureaupb.BureauService_ServiceDesc.ServiceName,
	})

	if err != nil {
		return err
	}
	if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return errors.New("bureau service is not serving")
	}

	return nil
}

func run(ctx context.Context, logger *slog.Logger) error {
	address := envOr("BUREAU_GRPC_ADDR", ":50053")
	seedPath := envOr("BUREAU_PERSONAS_PATH", "seed/personas.yaml")
	catalog, err := persona.Load(seedPath)
	if err != nil {
		return err
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("listen for bureau gRPC: %w", err)
	}
	defer func() { _ = listener.Close() }()

	server, healthServer := grpcserver.NewServer(catalog, os.Getenv("BUREAU_ENABLE_REFLECTION") == "true")
	serveErrors := make(chan error, 1)

	go func() { serveErrors <- server.Serve(listener) }()
	logger.Info("bureau-mocks started", "address", address)
	select {
	case err := <-serveErrors:
		return fmt.Errorf("serve bureau gRPC: %w", err)
	case <-ctx.Done():
	}

	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	healthServer.SetServingStatus(bureaupb.BureauService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	stopped := make(chan struct{})
	go func() { server.GracefulStop(); close(stopped) }()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		server.Stop()
		<-stopped
	}
	if err := <-serveErrors; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("stop bureau gRPC: %w", err)
	}
	logger.Info("bureau-mocks stopped")
	return nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
