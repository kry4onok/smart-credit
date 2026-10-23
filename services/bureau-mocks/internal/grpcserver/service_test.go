package grpcserver

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bureau-mocks/internal/persona"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func testService(t *testing.T) *Service {
	t.Helper()
	catalog, err := persona.Load(filepath.Join("..", "..", "..", "..", "seed", "personas.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return New(catalog)
}

func TestSuccessAndFailureScenarios(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	profile, err := service.GetCompanyProfile(ctx, &bureaupb.CompanyProfileRequest{Inn: "7707083893"})
	if err != nil || profile.GetRegisteredAt() != "2019-04-15" || profile.GetStatus() != bureaupb.CompanyStatus_COMPANY_STATUS_ACTIVE {
		t.Fatalf("company profile = %v, %v", profile, err)
	}
	credit, err := service.GetCreditHistory(ctx, &bureaupb.CreditHistoryRequest{Inn: "7707083893"})
	if err != nil || credit.GetTotalDebt().GetAmountKopecks() != 120000000 || credit.OverdueCount_12M == nil || *credit.OverdueCount_12M != 0 {
		t.Fatalf("credit history = %v, %v", credit, err)
	}
	_, err = service.GetCreditHistory(ctx, &bureaupb.CreditHistoryRequest{Inn: "1234567894"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("configured credit failure = %v", err)
	}
	_, err = service.GetCompanyProfile(ctx, &bureaupb.CompanyProfileRequest{Inn: "1234567894"})
	if err != nil {
		t.Fatalf("credit failure must not affect company profile: %v", err)
	}
	_, err = service.GetCompanyProfile(ctx, &bureaupb.CompanyProfileRequest{Inn: "1234567809"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown valid INN = %v", err)
	}
	_, err = service.GetCreditHistory(ctx, &bureaupb.CreditHistoryRequest{Inn: "7707083894"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid INN = %v", err)
	}
}

func TestDelayedCreditRespectsDeadline(t *testing.T) {
	service := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := service.GetCreditHistory(ctx, &bureaupb.CreditHistoryRequest{Inn: "5401010012"})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("delayed credit = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("deadline was not honored promptly")
	}
}

func TestGRPCTransportAndHealth(t *testing.T) {
	catalog, err := persona.Load(filepath.Join("..", "..", "..", "..", "seed", "personas.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	server, _ := NewServer(catalog, false)
	listener := bufconn.Listen(1024 * 1024)
	defer func() { _ = listener.Close() }()
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	connection, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	health, err := healthpb.NewHealthClient(connection).Check(ctx, &healthpb.HealthCheckRequest{
		Service: bureaupb.BureauService_ServiceDesc.ServiceName,
	})
	if err != nil || health.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health = %v, %v", health, err)
	}
	credit, err := bureaupb.NewBureauServiceClient(connection).GetCreditHistory(ctx, &bureaupb.CreditHistoryRequest{Inn: "9876543210"})
	if err != nil || credit.OverdueCount_12M == nil || *credit.OverdueCount_12M != 0 {
		t.Fatalf("wire credit history = %v, %v", credit, err)
	}
	missing, err := bureaupb.NewBureauServiceClient(connection).GetCreditHistory(ctx, &bureaupb.CreditHistoryRequest{Inn: "500100732259"})
	if err != nil || missing.TotalDebt != nil || missing.OverdueCount_12M != nil || missing.MaxOverdueDays != nil {
		t.Fatalf("wire missing credit fields = %v, %v", missing, err)
	}
}
