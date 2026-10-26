package bureau

import (
	"context"
	"errors"
	"testing"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type rpcStub struct {
	profileCalls int
	creditCalls  int
	profile      func() (*bureaupb.CompanyProfileResponse, error)
	credit       func() (*bureaupb.CreditHistoryResponse, error)
}

func (s *rpcStub) GetCompanyProfile(context.Context, *bureaupb.CompanyProfileRequest, ...grpc.CallOption) (*bureaupb.CompanyProfileResponse, error) {
	s.profileCalls++
	return s.profile()
}

func (s *rpcStub) GetCreditHistory(context.Context, *bureaupb.CreditHistoryRequest, ...grpc.CallOption) (*bureaupb.CreditHistoryResponse, error) {
	s.creditCalls++
	return s.credit()
}

func TestRetryUnavailableOnce(t *testing.T) {
	rpc := &rpcStub{}
	rpc.profile = func() (*bureaupb.CompanyProfileResponse, error) {
		if rpc.profileCalls == 1 {
			return nil, status.Error(codes.Unavailable, "temporary")
		}
		return &bureaupb.CompanyProfileResponse{
			Status:       bureaupb.CompanyStatus_COMPANY_STATUS_ACTIVE,
			RegisteredAt: "2019-04-15", Okved: "10.71",
		}, nil
	}
	value, err := New(rpc).GetCompanyProfile(t.Context(), "7707083893")
	if err != nil || value.GetRegisteredAt() != "2019-04-15" || rpc.profileCalls != 2 {
		t.Fatalf("profile=%v error=%v calls=%d", value, err, rpc.profileCalls)
	}
}

func TestInvalidResponseAndKnownZero(t *testing.T) {
	zero := int32(0)
	rpc := &rpcStub{credit: func() (*bureaupb.CreditHistoryResponse, error) {
		return &bureaupb.CreditHistoryResponse{
			TotalDebt:        &commonpb.Money{AmountKopecks: 0, Currency: "RUB"},
			OverdueCount_12M: &zero, MaxOverdueDays: &zero,
		}, nil
	}}
	value, err := New(rpc).GetCreditHistory(t.Context(), "9876543210")
	if err != nil || value.TotalDebt == nil || value.OverdueCount_12M == nil || *value.OverdueCount_12M != 0 {
		t.Fatalf("known zero lost: %v, %v", value, err)
	}
	rpc.credit = func() (*bureaupb.CreditHistoryResponse, error) {
		return &bureaupb.CreditHistoryResponse{TotalDebt: &commonpb.Money{Currency: "USD"}}, nil
	}
	_, err = New(rpc).GetCreditHistory(t.Context(), "9876543210")
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("invalid response = %v", err)
	}
}

func TestDoesNotRetryNotFound(t *testing.T) {
	rpc := &rpcStub{profile: func() (*bureaupb.CompanyProfileResponse, error) {
		return nil, status.Error(codes.NotFound, "unknown")
	}}
	_, err := New(rpc).GetCompanyProfile(t.Context(), "1234567809")
	if status.Code(err) != codes.NotFound || rpc.profileCalls != 1 {
		t.Fatalf("error=%v calls=%d", err, rpc.profileCalls)
	}
}

func (r *rpcStub) GetAssessmentSources(context.Context, *bureaupb.AssessmentSourcesRequest, ...grpc.CallOption) (*bureaupb.AssessmentSourcesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "test stub")
}
