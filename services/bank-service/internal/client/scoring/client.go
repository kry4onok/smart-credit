package scoring

import (
	"context"
	"errors"
	"fmt"
	"time"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const requestTimeout = 2 * time.Second

type Client struct {
	rpc        scoringpb.ScoringServiceClient
	connection *grpc.ClientConn
}

func New(rpc scoringpb.ScoringServiceClient) *Client {
	return &Client{rpc: rpc}
}

func Dial(target string) (*Client, error) {
	if target == "" {
		return nil, errors.New("scoring target is required")
	}

	connection, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create scoring connection: %w", err)
	}
	return &Client{rpc: scoringpb.NewScoringServiceClient(connection), connection: connection}, nil
}

func (c *Client) Close() error {
	if c == nil || c.connection == nil {
		return nil
	}
	return c.connection.Close()
}

func (c *Client) Score(ctx context.Context, input Input) (*scoringpb.ScoreResponse, error) {
	request, err := BuildRequest(input)
	if err != nil {
		return nil, fmt.Errorf("build scoring request: %w", err)
	}
	if c == nil || c.rpc == nil {
		return nil, errors.New("scoring client is not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	for attempt := 0; attempt < 2; attempt++ {
		response, rpcErr := c.rpc.Score(ctx, request)
		if rpcErr == nil {
			if err := ValidateResponse(response); err != nil {
				return nil, err
			}
			return response, nil
		}
		if status.Code(rpcErr) != codes.Unavailable || attempt == 1 || ctx.Err() != nil {
			return nil, fmt.Errorf("scoring RPC (%s): %w", status.Code(rpcErr), rpcErr)
		}
	}
	return nil, errors.New("scoring RPC exhausted retries")
}
