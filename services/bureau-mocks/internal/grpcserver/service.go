package grpcserver

import (
	"context"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bureau-mocks/internal/persona"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Service struct {
	bureaupb.UnimplementedBureauServiceServer
	catalog *persona.Catalog
}

func New(catalog *persona.Catalog) *Service {
	return &Service{catalog: catalog}
}

func (s *Service) GetCompanyProfile(ctx context.Context, req *bureaupb.CompanyProfileRequest) (*bureaupb.CompanyProfileResponse, error) {
	if req == nil || !persona.ValidINN(req.GetInn()) {
		return nil, status.Error(codes.InvalidArgument, "valid INN required")
	}
	item, ok := s.catalog.Lookup(req.GetInn())
	if !ok {
		return nil, status.Error(codes.NotFound, "persona not found")
	}
	if err := wait(ctx, item.ProfileDelay); err != nil {
		return nil, err
	}
	if item.ProfileError {
		return nil, status.Error(codes.Unavailable, "company registry unavailable")
	}
	return proto.Clone(item.Profile).(*bureaupb.CompanyProfileResponse), nil
}

func (s *Service) GetCreditHistory(ctx context.Context, req *bureaupb.CreditHistoryRequest) (*bureaupb.CreditHistoryResponse, error) {
	if req == nil || !persona.ValidINN(req.GetInn()) {
		return nil, status.Error(codes.InvalidArgument, "valid INN required")
	}
	item, ok := s.catalog.Lookup(req.GetInn())
	if !ok {
		return nil, status.Error(codes.NotFound, "persona not found")
	}

	if err := wait(ctx, item.CreditDelay); err != nil {
		return nil, err
	}

	if item.CreditError {
		return nil, status.Error(codes.Unavailable, "credit bureau unavailable")
	}
	return proto.Clone(item.Credit).(*bureaupb.CreditHistoryResponse), nil
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay == 0 {
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return status.FromContextError(ctx.Err()).Err()
	case <-timer.C:
		return nil
	}
}
