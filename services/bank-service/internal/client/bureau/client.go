package bureau

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const requestTimeout = 2 * time.Second

var ErrInvalidResponse = errors.New("invalid bureau response")

type Client struct {
	rpc        bureaupb.BureauServiceClient
	connection *grpc.ClientConn
}

func New(rpc bureaupb.BureauServiceClient) *Client {
	return &Client{rpc: rpc}
}

func Dial(target string) (*Client, error) {
	if target == "" {
		return nil, errors.New("bureau target is required")
	}
	connection, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create bureau connection: %w", err)
	}
	return &Client{rpc: bureaupb.NewBureauServiceClient(connection), connection: connection}, nil
}

func (c *Client) Close() error {
	if c == nil || c.connection == nil {
		return nil
	}
	return c.connection.Close()
}

func (c *Client) GetCompanyProfile(ctx context.Context, inn string) (*bureaupb.CompanyProfileResponse, error) {
	if c == nil || c.rpc == nil {
		return nil, errors.New("bureau client is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var response *bureaupb.CompanyProfileResponse
	err := retryUnavailable(ctx, func() error {
		var callErr error
		response, callErr = c.rpc.GetCompanyProfile(ctx, &bureaupb.CompanyProfileRequest{Inn: inn})
		return callErr
	})
	if err != nil {
		return nil, fmt.Errorf("get company profile: %w", err)
	}
	if err := validateProfile(response); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidResponse, err)
	}
	return response, nil
}

func (c *Client) GetCreditHistory(ctx context.Context, inn string) (*bureaupb.CreditHistoryResponse, error) {
	if c == nil || c.rpc == nil {
		return nil, errors.New("bureau client is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var response *bureaupb.CreditHistoryResponse
	err := retryUnavailable(ctx, func() error {
		var callErr error
		response, callErr = c.rpc.GetCreditHistory(ctx, &bureaupb.CreditHistoryRequest{Inn: inn})
		return callErr
	})
	if err != nil {
		return nil, fmt.Errorf("get credit history: %w", err)
	}
	if err := validateCredit(response); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidResponse, err)
	}
	return response, nil
}

func retryUnavailable(ctx context.Context, call func() error) error {
	for attempt := 0; attempt < 2; attempt++ {
		err := call()
		if err == nil {
			return nil
		}
		if status.Code(err) != codes.Unavailable || attempt == 1 || ctx.Err() != nil {
			return err
		}
	}
	return errors.New("bureau RPC exhausted retries")
}

func validateProfile(value *bureaupb.CompanyProfileResponse) error {
	if value == nil || value.Status == bureaupb.CompanyStatus_COMPANY_STATUS_UNSPECIFIED ||
		value.Status < bureaupb.CompanyStatus_COMPANY_STATUS_ACTIVE ||
		value.Status > bureaupb.CompanyStatus_COMPANY_STATUS_LIQUIDATED || value.Okved == "" {
		return errors.New("missing company status or OKVED")
	}
	date, err := time.Parse(time.DateOnly, value.RegisteredAt)
	if err != nil || date.Format(time.DateOnly) != value.RegisteredAt {
		return errors.New("invalid registration date")
	}
	return nil
}

func validateCredit(value *bureaupb.CreditHistoryResponse) error {
	if value == nil {
		return errors.New("missing credit history")
	}
	if value.TotalDebt != nil && (value.TotalDebt.Currency != "RUB" || value.TotalDebt.AmountKopecks < 0) {
		return errors.New("invalid total debt")
	}
	if value.OverdueCount_12M != nil && *value.OverdueCount_12M < 0 ||
		value.MaxOverdueDays != nil && *value.MaxOverdueDays < 0 {
		return errors.New("negative overdue values")
	}
	for _, obligation := range value.Obligations {
		if obligation == nil || obligation.Kind == "" || obligation.Outstanding == nil ||
			obligation.Outstanding.Currency != "RUB" || obligation.Outstanding.AmountKopecks < 0 {
			return errors.New("invalid obligation")
		}
	}
	return nil
}

func (c *Client) GetAssessmentSources(ctx context.Context, inn string, asOf time.Time) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, err := c.rpc.GetAssessmentSources(ctx, &bureaupb.AssessmentSourcesRequest{Inn: inn, AsOf: asOf.Format(time.RFC3339Nano)})
	if err != nil {
		return nil, err
	}
	if response == nil || len(response.SourcesJson) == 0 || len(response.SourcesJson) > 256*1024 {
		return nil, ErrInvalidResponse
	}
	var source assessment.Sources
	decoder := json.NewDecoder(bytes.NewReader(response.SourcesJson))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&source); err != nil || source.SchemaVersion != 1 || source.INN != inn || len(source.Checks) > 60 {
		return nil, ErrInvalidResponse
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidResponse
	}
	return response.SourcesJson, nil
}
