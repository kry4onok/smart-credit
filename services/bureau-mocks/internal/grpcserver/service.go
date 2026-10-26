package grpcserver

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"

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

func (s *Service) GetAssessmentSources(ctx context.Context, req *bureaupb.AssessmentSourcesRequest) (*bureaupb.AssessmentSourcesResponse, error) {
	if req == nil || !persona.ValidINN(req.Inn) {
		return nil, status.Error(codes.InvalidArgument, "valid INN required")
	}
	now, err := time.Parse(time.RFC3339Nano, req.AsOf)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "as_of required")
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	item, ok := s.catalog.Lookup(req.Inn)
	if !ok {
		return nil, status.Error(codes.NotFound, "persona not found")
	}
	profile, credit := item.Profile, item.Credit
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(-1, 0, 0)
	current := 0
	history := assessment.CreditHistory{CurrentDPD: &current, PeriodFrom: start.Format(time.DateOnly), PeriodTo: start.AddDate(1, 0, -1).Format(time.DateOnly)}
	if credit.MaxOverdueDays != nil {
		v := int(*credit.MaxOverdueDays)
		history.HistoricalMaxDPD = &v
	}
	if credit.OverdueCount_12M != nil {
		v := int(*credit.OverdueCount_12M)
		history.OverdueEpisodes = &v
	}
	if credit.TotalDebt != nil {
		v := credit.TotalDebt.AmountKopecks
		history.TotalDebtKopecks = &v
	}
	sources := assessment.DemoSources(req.Inn, profile.RegisteredAt, strings.TrimPrefix(profile.Status.String(), "COMPANY_STATUS_"), history, now)
	for i := range sources.Checks {
		source := &sources.Checks[i]
		if source.Kind == "REGISTRY" && (item.ProfileError || item.ProfileDelay >= 2*time.Second) {
			source.Status = "UNAVAILABLE"
			source.Conclusion = "REVIEW"
			sources.RegisteredAt = ""
			sources.CompanyStatus = ""
		}
		if source.Kind == "CREDIT_HISTORY" && (item.CreditError || item.CreditDelay >= 2*time.Second) {
			source.Status = "UNAVAILABLE"
			source.Conclusion = "REVIEW"
			sources.Credit = assessment.CreditHistory{}
		}
	}
	data, err := json.Marshal(sources)
	if err != nil {
		return nil, status.Error(codes.Internal, "encode sources")
	}
	return &bureaupb.AssessmentSourcesResponse{SourcesJson: data}, nil
}
