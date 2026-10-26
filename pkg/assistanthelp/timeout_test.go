package assistanthelp

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/assistant"
	"google.golang.org/grpc"
)

type deadlineProbe struct {
	pb.UnimplementedAssistantHelpServiceServer
	remaining chan time.Duration
}

func (p *deadlineProbe) Help(ctx context.Context, request *pb.HelpRequest) (*pb.HelpResponse, error) {
	deadline, _ := ctx.Deadline()
	p.remaining <- time.Until(deadline)
	return Fallback(request, pb.FallbackReason_FALLBACK_REASON_MODEL_UNAVAILABLE, time.Now().UTC())
}

func TestCallPropagatesConfiguredDeadline(t *testing.T) {
	for _, tc := range []struct {
		name   string
		value  string
		parent time.Duration
		want   time.Duration
	}{
		{name: "default", want: 5 * time.Second},
		{name: "cpu model", value: "45s", want: 45 * time.Second},
		{name: "invalid", value: "invalid", want: 5 * time.Second},
		{name: "zero", value: "0s", want: 5 * time.Second},
		{name: "negative", value: "-1s", want: 5 * time.Second},
		{name: "exceeds HTTP budget", value: "60s", want: 5 * time.Second},
		{name: "parent deadline", value: "45s", parent: 2 * time.Second, want: 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASSISTANT_HELP_TIMEOUT", tc.value)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			server := grpc.NewServer()
			probe := &deadlineProbe{remaining: make(chan time.Duration, 1)}
			pb.RegisterAssistantHelpServiceServer(server, probe)
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(server.Stop)
			t.Cleanup(func() { _ = listener.Close() })
			ctx := t.Context()
			if tc.parent > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.parent)
				defer cancel()
			}
			request := &pb.HelpRequest{
				RequestId: "11111111-1111-4111-8111-111111111111",
				Context: Context(View{
					ApplicationID: "22222222-2222-4222-8222-222222222222",
					Revision:      "1", Status: "DRAFT",
				}, pb.Audience_AUDIENCE_CLIENT, time.Now().UTC()),
				Mode: pb.HelpMode_HELP_MODE_NEXT_STEP, Detail: pb.Detail_DETAIL_CONCISE, Locale: "ru-RU",
			}
			if _, err := Call(ctx, listener.Addr().String(), request); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-probe.remaining:
				if got > tc.want+100*time.Millisecond || got < tc.want-500*time.Millisecond {
					t.Fatalf("RPC deadline remaining = %s, want approximately %s", got, tc.want)
				}
			default:
				t.Fatal("Help RPC was not called")
			}
		})
	}
}
