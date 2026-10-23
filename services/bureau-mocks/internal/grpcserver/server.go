package grpcserver

import (
	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bureau-mocks/internal/persona"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func NewServer(catalog *persona.Catalog, enableReflection bool) (*grpc.Server, *health.Server) {
	server := grpc.NewServer()
	bureaupb.RegisterBureauServiceServer(server, New(catalog))
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(bureaupb.BureauService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	if enableReflection {
		reflection.Register(server)
	}
	return server, healthServer
}
