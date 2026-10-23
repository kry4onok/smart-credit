package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func main() {
	target := flag.String("target", "127.0.0.1:50053", "bureau gRPC endpoint")
	flag.Parse()
	if err := run(*target); err != nil {
		fmt.Fprintln(os.Stderr, "bureau smoke failed:", err)
		os.Exit(1)
	}
	fmt.Println("bureau smoke passed: health, profile, credit, configured error, unknown INN")
}

func run(target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connection, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	health, err := healthpb.NewHealthClient(connection).Check(ctx, &healthpb.HealthCheckRequest{
		Service: bureaupb.BureauService_ServiceDesc.ServiceName,
	})
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	if health.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return errors.New("bureau health is not SERVING")
	}

	client := bureaupb.NewBureauServiceClient(connection)
	profile, err := client.GetCompanyProfile(ctx, &bureaupb.CompanyProfileRequest{Inn: "7707083893"})
	if err != nil {
		return fmt.Errorf("company profile: %w", err)
	}
	if profile.GetRegisteredAt() != "2019-04-15" {
		return errors.New("company profile mismatch")
	}

	credit, err := client.GetCreditHistory(ctx, &bureaupb.CreditHistoryRequest{Inn: "7707083893"})
	if err != nil {
		return fmt.Errorf("credit history: %w", err)
	}
	if credit.GetTotalDebt().GetAmountKopecks() != 120000000 || credit.OverdueCount_12M == nil {
		return errors.New("credit history mismatch")
	}
	_, err = client.GetCreditHistory(ctx, &bureaupb.CreditHistoryRequest{Inn: "1234567894"})
	if status.Code(err) != codes.Unavailable {
		if err != nil {
			return fmt.Errorf("configured bureau failure has unexpected code: %w", err)
		}
		return errors.New("configured bureau failure returned success")
	}
	_, err = client.GetCompanyProfile(ctx, &bureaupb.CompanyProfileRequest{Inn: "1234567809"})
	if status.Code(err) != codes.NotFound {
		if err != nil {
			return fmt.Errorf("unknown INN has unexpected code: %w", err)
		}
		return errors.New("unknown INN returned success")
	}
	return nil
}
