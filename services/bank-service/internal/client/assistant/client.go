package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	assistantpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/assistant"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/summary"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultRequestTimeout = 5 * time.Second

type RPC interface {
	SummarizeCase(context.Context, *assistantpb.SummarizeCaseRequest, ...grpc.CallOption) (*assistantpb.TextResponse, error)
}

type Client struct {
	rpc        RPC
	connection *grpc.ClientConn
	timeout    time.Duration
}

func New(rpc RPC) *Client {
	return &Client{rpc: rpc, timeout: defaultRequestTimeout}
}

func Dial(target string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(target) == "" {
		return nil, errors.New("assistant target is required")
	}
	if timeout <= 0 {
		return nil, errors.New("assistant timeout must be positive")
	}
	connection, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create assistant connection: %w", err)
	}
	return &Client{rpc: assistantpb.NewAssistantServiceClient(connection), connection: connection, timeout: timeout}, nil
}

func (c *Client) Close() error {
	if c == nil || c.connection == nil {
		return nil
	}
	return c.connection.Close()
}

func (c *Client) Summarize(ctx context.Context, facts summary.Facts) (string, string, error) {
	if c == nil || c.rpc == nil {
		return "", "", errors.New("assistant client is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.SummarizeCase(ctx, &assistantpb.SummarizeCaseRequest{
		ApplicationId: facts.ApplicationID, ProductName: facts.ProductName,
		Purpose: facts.Purpose, Grade: facts.Grade,
		ReasonCodes: facts.ReasonCodes, TriggeredRuleCodes: facts.TriggeredRuleCodes,
	})
	if err != nil {
		return "", "", fmt.Errorf("summarize case RPC: %w", err)
	}
	if response == nil || strings.TrimSpace(response.GetText()) == "" {
		return "", "", errors.New("assistant returned empty summary")
	}
	source := "assistant"
	if response.GetModelVersion() == "" || response.GetModelVersion() == "template" {
		source = "template"
	}
	return response.GetText(), source, nil
}
